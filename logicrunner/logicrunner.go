/*
 *    Copyright 2018 Insolar
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

// Package logicrunner - infrastructure for executing smartcontracts
package logicrunner

import (
	"bytes"
	"context"
	"encoding/gob"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/insolar/insolar/instrumentation/instracer"
	"go.opencensus.io/trace"

	"github.com/pkg/errors"

	"github.com/insolar/insolar/configuration"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/message"
	"github.com/insolar/insolar/core/reply"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/logicrunner/builtin"
	"github.com/insolar/insolar/logicrunner/goplugin"
)

type Ref = core.RecordRef

// Context of one contract execution
type ObjectState struct {
	sync.Mutex
	Ref *Ref

	ExecutionState *ExecutionState
	Validation     *ExecutionState
	Consensus      *Consensus
}

type ExecutionState struct {
	sync.Mutex

	ArtifactManager core.ArtifactManager

	objectbody *ObjectBody
	deactivate bool
	nonce      uint64

	Behaviour ValidationBehaviour

	Current              *CurrentExecution
	Queue                []ExecutionQueueElement
	QueueProcessorActive bool

	// TODO not using in validation, need separate ObjectState.ExecutionState and ObjectState.Validation from ExecutionState struct
	pending          message.PendingState
	PendingConfirmed bool
}

type CurrentExecution struct {
	Context       context.Context
	LogicContext  *core.LogicCallContext
	Request       *Ref
	Sequence      uint64
	RequesterNode *Ref
	ReturnMode    message.MethodReturnMode
	SentResult    bool
}

type ExecutionQueueResult struct {
	reply        core.Reply
	err          error
	somebodyElse bool
}

type ExecutionQueueElement struct {
	ctx     context.Context
	parcel  core.Parcel
	request *Ref
	pulse   core.PulseNumber
}

type Error struct {
	Err      error
	Request  *Ref
	Contract *Ref
	Method   string
}

func (lre Error) Error() string {
	var buffer bytes.Buffer

	buffer.WriteString(lre.Err.Error())
	if lre.Contract != nil {
		buffer.WriteString(" Contract=" + lre.Contract.String())
	}
	if lre.Method != "" {
		buffer.WriteString(" Method=" + lre.Method)
	}
	if lre.Request != nil {
		buffer.WriteString(" Request=" + lre.Request.String())
	}

	return buffer.String()
}

func (st *ObjectState) MustModeState(mode string) (res *ExecutionState) {
	switch mode {
	case "execution":
		res = st.ExecutionState
	case "validation":
		res = st.Validation
	default:
		panic("'" + mode + "' is unknown object processing mode")
	}
	if res == nil {
		panic("object is not in " + mode + " mode")
	}
	return res
}

func (st *ObjectState) WrapError(err error, message string) error {
	if err == nil {
		err = errors.New(message)
	} else {
		err = errors.Wrap(err, message)
	}
	return Error{
		Err:      err,
		Contract: st.Ref,
	}
}

func (es *ExecutionState) WrapError(err error, message string) error {
	if err == nil {
		err = errors.New(message)
	} else {
		err = errors.Wrap(err, message)
	}
	res := Error{Err: err}
	if es.objectbody != nil {
		res.Contract = es.objectbody.objDescriptor.HeadRef()
	}
	if es.Current != nil {
		res.Request = es.Current.Request
	}
	return res
}

func (es *ExecutionState) CheckPendingRequests(ctx context.Context, inMsg core.Message) (message.PendingState, error) {
	msg, ok := inMsg.(*message.CallMethod)
	if !ok {
		return message.NotPending, nil
	}

	has, err := es.ArtifactManager.HasPendingRequests(ctx, msg.ObjectRef)
	if err != nil {
		return message.NotPending, err
	}
	if has {
		return message.InPending, nil
	}

	return message.NotPending, nil
}

// releaseQueue must be calling only with es.Lock
func (es *ExecutionState) releaseQueue() []ExecutionQueueElement {
	q := es.Queue
	es.Queue = make([]ExecutionQueueElement, 0)

	return q
}

// LogicRunner is a general interface of contract executor
type LogicRunner struct {
	// FIXME: Ledger component is deprecated. Inject required sub-components.
	MessageBus                 core.MessageBus                 `inject:""`
	ContractRequester          core.ContractRequester          `inject:""`
	Ledger                     core.Ledger                     `inject:""`
	NodeNetwork                core.NodeNetwork                `inject:""`
	PlatformCryptographyScheme core.PlatformCryptographyScheme `inject:""`
	ParcelFactory              message.ParcelFactory           `inject:""`
	PulseStorage               core.PulseStorage               `inject:""`
	ArtifactManager            core.ArtifactManager            `inject:""`
	JetCoordinator             core.JetCoordinator             `inject:""`

	Executors    [core.MachineTypesLastID]core.MachineLogicExecutor
	machinePrefs []core.MachineType
	Cfg          *configuration.LogicRunner

	state      map[Ref]*ObjectState // if object exists, we are validating or executing it right now
	stateMutex sync.RWMutex

	sock net.Listener
}

// NewLogicRunner is constructor for LogicRunner
func NewLogicRunner(cfg *configuration.LogicRunner) (*LogicRunner, error) {
	if cfg == nil {
		return nil, errors.New("LogicRunner have nil configuration")
	}
	res := LogicRunner{
		Cfg:   cfg,
		state: make(map[Ref]*ObjectState),
	}
	return &res, nil
}

// Start starts logic runner component
func (lr *LogicRunner) Start(ctx context.Context) error {
	if lr.Cfg.BuiltIn != nil {
		bi := builtin.NewBuiltIn(lr.MessageBus, lr.ArtifactManager)
		if err := lr.RegisterExecutor(core.MachineTypeBuiltin, bi); err != nil {
			return err
		}
		lr.machinePrefs = append(lr.machinePrefs, core.MachineTypeBuiltin)
	}

	if lr.Cfg.GoPlugin != nil {
		if lr.Cfg.RPCListen != "" {
			StartRPC(ctx, lr, lr.PulseStorage)
		}

		gp, err := goplugin.NewGoPlugin(lr.Cfg, lr.MessageBus, lr.ArtifactManager)
		if err != nil {
			return err
		}
		if err := lr.RegisterExecutor(core.MachineTypeGoPlugin, gp); err != nil {
			return err
		}
		lr.machinePrefs = append(lr.machinePrefs, core.MachineTypeGoPlugin)
	}

	lr.RegisterHandlers()

	return nil
}

func (lr *LogicRunner) RegisterHandlers() {
	lr.MessageBus.MustRegister(core.TypeCallMethod, lr.Execute)
	lr.MessageBus.MustRegister(core.TypeCallConstructor, lr.Execute)
	lr.MessageBus.MustRegister(core.TypeExecutorResults, lr.HandleExecutorResultsMessage)
	lr.MessageBus.MustRegister(core.TypeValidateCaseBind, lr.HandleValidateCaseBindMessage)
	lr.MessageBus.MustRegister(core.TypeValidationResults, lr.HandleValidationResultsMessage)
	lr.MessageBus.MustRegister(core.TypePendingFinished, lr.HandlePendingFinishedMessage)
	lr.MessageBus.MustRegister(core.TypeStillExecuting, lr.HandleStillExecutingMessage)
}

// Stop stops logic runner component and its executors
func (lr *LogicRunner) Stop(ctx context.Context) error {
	reterr := error(nil)
	for _, e := range lr.Executors {
		if e == nil {
			continue
		}
		err := e.Stop()
		if err != nil {
			reterr = errors.Wrap(reterr, err.Error())
		}
	}

	if lr.sock != nil {
		if err := lr.sock.Close(); err != nil {
			return err
		}
	}

	return reterr
}

func (lr *LogicRunner) CheckOurRole(ctx context.Context, msg core.Message, role core.DynamicRole) error {
	// TODO do map of supported objects for pulse, go to jetCoordinator only if map is empty for ref
	target := msg.DefaultTarget()
	isAuthorized, err := lr.JetCoordinator.IsAuthorized(
		ctx, role, *target.Record(), lr.pulse(ctx).PulseNumber, lr.JetCoordinator.Me(),
	)
	if err != nil {
		return errors.Wrap(err, "authorization failed with error")
	}
	if !isAuthorized {
		return errors.New("can't execute this object")
	}
	return nil
}

func (lr *LogicRunner) RegisterRequest(ctx context.Context, parcel core.Parcel) (*Ref, error) {
	ctx, span := instracer.StartSpan(ctx, "LogicRunner.RegisterRequest")
	defer span.End()

	obj := parcel.Message().(message.IBaseLogicMessage).GetReference()
	id, err := lr.ArtifactManager.RegisterRequest(ctx, obj, parcel)
	if err != nil {
		return nil, err
	}

	res := obj
	res.SetRecord(*id)
	return &res, nil
}

// Execute runs a method on an object, ATM just thin proxy to `GoPlugin.Exec`
func (lr *LogicRunner) Execute(ctx context.Context, parcel core.Parcel) (core.Reply, error) {
	msg, ok := parcel.Message().(message.IBaseLogicMessage)
	if !ok {
		return nil, errors.New("Execute( ! message.IBaseLogicMessage )")
	}

	ctx, span := instracer.StartSpan(ctx, "LogicRunner.Execute")
	span.AddAttributes(
		trace.StringAttribute("msg.Type", msg.Type().String()),
	)
	defer span.End()

	rep, err := lr.executeActual(ctx, parcel, msg)
	return rep, err
}

func (lr *LogicRunner) executeActual(ctx context.Context, parcel core.Parcel, msg message.IBaseLogicMessage) (core.Reply, error) {

	ref := msg.GetReference()
	os := lr.UpsertObjectState(ref)

	os.Lock()
	if os.ExecutionState == nil {
		os.ExecutionState = &ExecutionState{
			ArtifactManager: lr.ArtifactManager,
			Queue:           make([]ExecutionQueueElement, 0),
			Behaviour:       &ValidationSaver{lr: lr, caseBind: NewCaseBind()},
		}
	}
	es := os.ExecutionState
	os.Unlock()

	// ExecutionState should be locked between CheckOurRole and
	// appending ExecutionQueueElement to the queue to prevent a race condition.
	// Otherwise it's possible that OnPulse will clean up the queue and set
	// ExecutionState.Pending to NotPending. Execute will add an element to the
	// queue afterwards. In this case cross-pulse execution will break.
	es.Lock()

	err := lr.CheckOurRole(ctx, msg, core.DynamicRoleVirtualExecutor)
	if err != nil {
		es.Unlock()
		return nil, errors.Wrap(err, "[ Execute ] can't play role")
	}

	if lr.CheckExecutionLoop(ctx, es, parcel) {
		es.Unlock()
		return nil, os.WrapError(nil, "loop detected")
	}

	request, err := lr.RegisterRequest(ctx, parcel)
	if err != nil {
		es.Unlock()
		return nil, os.WrapError(err, "[ Execute ] can't create request")
	}

	_, span := instracer.StartSpan(ctx, "LogicRunner.QueueCall")

	// Attention! Do not refactor this line if no sure. Here is no bug. Many specialists spend lots of time
	// to write it as it is.
	span.End()

	qElement := ExecutionQueueElement{
		ctx:     ctx,
		parcel:  parcel,
		request: request,
		pulse:   lr.pulse(ctx).PulseNumber,
	}

	es.Queue = append(es.Queue, qElement)
	es.Unlock()

	err = lr.StartQueueProcessorIfNeeded(ctx, es, msg)
	if err != nil {
		return nil, err
	}

	return &reply.RegisterRequest{
		Request: *request,
	}, nil
}

func (lr *LogicRunner) CheckExecutionLoop(
	ctx context.Context, es *ExecutionState, parcel core.Parcel,
) bool {
	if es.Current == nil {
		return false
	}

	if es.Current.SentResult {
		return false
	}

	if es.Current.ReturnMode == message.ReturnNoWait {
		return false
	}

	msg, ok := parcel.Message().(*message.CallMethod)
	if ok && msg.ReturnMode == message.ReturnNoWait {
		return false
	}

	if inslogger.TraceID(es.Current.Context) != inslogger.TraceID(ctx) {
		return false
	}

	inslogger.FromContext(ctx).Debug("loop detected")

	return true
}

func (lr *LogicRunner) HandlePendingFinishedMessage(
	ctx context.Context, parcel core.Parcel,
) (
	core.Reply, error,
) {
	msg := parcel.Message().(*message.PendingFinished)
	ref := msg.DefaultTarget()
	os := lr.UpsertObjectState(*ref)

	os.Lock()
	if os.ExecutionState == nil {
		// we are first, strange, soon ExecuteResults message should come
		os.ExecutionState = &ExecutionState{
			Queue:     make([]ExecutionQueueElement, 0),
			Behaviour: &ValidationSaver{lr: lr, caseBind: NewCaseBind()},
			pending:   message.NotPending,
		}
		os.Unlock()
		return &reply.OK{}, nil
	}
	es := os.ExecutionState
	os.Unlock()

	es.Lock()
	es.pending = message.NotPending
	if es.Current != nil {
		es.Unlock()
		return nil, errors.New("received PendingFinished when we are already executing")
	}
	es.Unlock()

	err := lr.StartQueueProcessorIfNeeded(ctx, es, parcel.Message())
	if err != nil {
		return nil, errors.Wrap(err, "couldn't start queue processor")
	}

	return &reply.OK{}, nil
}

func (lr *LogicRunner) StartQueueProcessorIfNeeded(
	ctx context.Context, es *ExecutionState, msg core.Message,
) error {
	es.Lock()
	defer es.Unlock()

	if len(es.Queue) == 0 {
		inslogger.FromContext(ctx).Debug("queue is empty. processor is not needed")
		return nil
	}

	if es.QueueProcessorActive {
		inslogger.FromContext(ctx).Debug("queue processor is already active. processor is not needed")
		return nil
	}

	if es.pending == message.PendingUnknown {
		pending, err := es.CheckPendingRequests(ctx, msg)
		if err != nil {
			return errors.Wrap(err, "couldn't check for pending requests")
		}
		es.pending = pending
	}
	if es.pending == message.InPending {
		inslogger.FromContext(ctx).Debug("object in pending. not starting queue processor")
		return nil
	}

	inslogger.FromContext(ctx).Debug("Starting a new queue processor")
	es.QueueProcessorActive = true
	go lr.ProcessExecutionQueue(ctx, es)
	return nil
}

func (lr *LogicRunner) ProcessExecutionQueue(ctx context.Context, es *ExecutionState) {
	for {
		es.Lock()
		q := es.Queue
		if len(q) == 0 {
			inslogger.FromContext(ctx).Debug("Quiting queue processing, empty")
			es.QueueProcessorActive = false
			es.Current = nil
			es.Unlock()
			return
		}
		qe, q := q[0], q[1:]
		es.Queue = q

		sender := qe.parcel.GetSender()
		current := CurrentExecution{
			Request:       qe.request,
			RequesterNode: &sender,
		}
		es.Current = &current

		if msg, ok := qe.parcel.Message().(*message.CallMethod); ok {
			current.ReturnMode = msg.ReturnMode
		}
		if msg, ok := qe.parcel.Message().(message.IBaseLogicMessage); ok {
			current.Sequence = msg.GetBaseLogicMessage().Sequence
		}

		es.Unlock()

		res := ExecutionQueueResult{}

		recordingBus := lr.MessageBus
		//recordingBus, err := lr.MessageBus.NewRecorder(qe.ctx, *lr.pulse(qe.ctx))
		//if err != nil {
		//	res.err = err
		//	continue
		//}

		current.Context = core.ContextWithMessageBus(qe.ctx, recordingBus)

		inslogger.FromContext(qe.ctx).Debug("Registering request within execution behaviour")
		es.Behaviour.(*ValidationSaver).NewRequest(qe.parcel, *qe.request, recordingBus)

		res.reply, res.err = lr.executeOrValidate(current.Context, es, qe.parcel)

		inslogger.FromContext(qe.ctx).Debug("Registering result within execution behaviour")
		err := es.Behaviour.Result(res.reply, res.err)
		if err != nil {
			res.err = err
		}

		lr.finishPendingIfNeeded(ctx, es, *qe.parcel.Message().DefaultTarget())
	}
}

// finishPendingIfNeeded checks whether last execution was a pending one.
// If this is true as a side effect the function sends a PendingFinished
// message to the current executor
func (lr *LogicRunner) finishPendingIfNeeded(ctx context.Context, es *ExecutionState, currentRef core.RecordRef) {
	es.Lock()
	defer es.Unlock()

	if es.pending != message.InPending {
		return
	}

	es.pending = message.NotPending
	es.PendingConfirmed = false

	pulse := lr.pulse(ctx)
	meCurrent, _ := lr.JetCoordinator.IsAuthorized(
		ctx, core.DynamicRoleVirtualExecutor, *currentRef.Record(), pulse.PulseNumber, lr.JetCoordinator.Me(),
	)
	if !meCurrent {
		es.objectbody = nil
		go func() {
			msg := message.PendingFinished{Reference: currentRef}
			_, err := lr.MessageBus.Send(ctx, &msg, nil)
			if err != nil {
				inslogger.FromContext(ctx).Error("Unable to send PendingFinished message:", err)
			}
		}()
	}
}

func (lr *LogicRunner) executeOrValidate(
	ctx context.Context, es *ExecutionState, parcel core.Parcel,
) (
	core.Reply, error,
) {
	ctx, span := instracer.StartSpan(ctx, "LogicRunner.ExecuteOrValidate")
	defer span.End()

	msg := parcel.Message().(message.IBaseLogicMessage)
	ref := msg.GetReference()

	es.Current.LogicContext = &core.LogicCallContext{
		Mode:            es.Behaviour.Mode(),
		Caller:          msg.GetCaller(),
		Callee:          &ref,
		Request:         es.Current.Request,
		Time:            time.Now(), // TODO: probably we should take it earlier
		Pulse:           *lr.pulse(ctx),
		TraceID:         inslogger.TraceID(ctx),
		CallerPrototype: msg.GetCallerPrototype(),
	}

	var re core.Reply
	var err error
	switch m := msg.(type) {
	case *message.CallMethod:
		re, err = lr.executeMethodCall(ctx, es, m)

	case *message.CallConstructor:
		re, err = lr.executeConstructorCall(ctx, es, m)

	default:
		panic("Unknown e type")
	}
	errstr := ""
	if err != nil {
		errstr = err.Error()
	}

	es.Lock()
	defer es.Unlock()

	es.Current.SentResult = true
	if es.Current.ReturnMode != message.ReturnResult {
		return re, err
	}

	target := *es.Current.RequesterNode
	request := *es.Current.Request
	seq := es.Current.Sequence

	go func() {
		inslogger.FromContext(ctx).Debugf("Sending Method Results for ", request)

		_, err := core.MessageBusFromContext(ctx, nil).Send(
			ctx,
			&message.ReturnResults{
				Caller:   lr.NodeNetwork.GetOrigin().ID(),
				Target:   target,
				Sequence: seq,
				Reply:    re,
				Error:    errstr,
			},
			&core.MessageSendOptions{
				Receiver: &target,
			},
		)
		if err != nil {
			inslogger.FromContext(ctx).Error("couldn't deliver results: ", err)
		}
	}()

	return re, err
}

// ObjectBody is an inner representation of object and all it accessory
// make it private again when we start it serialize before sending
type ObjectBody struct {
	objDescriptor   core.ObjectDescriptor
	Object          []byte
	Prototype       *Ref
	CodeMachineType core.MachineType
	CodeRef         *Ref
	Parent          *Ref
}

func init() {
	gob.Register(&ObjectBody{})
}

func (lr *LogicRunner) prepareObjectState(ctx context.Context, msg *message.ExecutorResults) error {
	state := lr.UpsertObjectState(msg.GetReference())
	state.Lock()
	if state.ExecutionState == nil {
		state.ExecutionState = &ExecutionState{
			ArtifactManager: lr.ArtifactManager,
			Queue:           make([]ExecutionQueueElement, 0),
			Behaviour:       &ValidationSaver{lr: lr, caseBind: NewCaseBind()},
		}
	}
	es := state.ExecutionState
	state.Unlock()

	es.Lock()

	if es.pending == message.InPending && es.Current != nil {
		inslogger.FromContext(ctx).Debug(
			"execution returned to node that is still executing pending",
		)
		es.pending = message.NotPending
		es.PendingConfirmed = false
	} else if es.pending == message.InPending && msg.Pending == message.NotPending {
		inslogger.FromContext(ctx).Debug(
			"executor we came to thinks that execution pending, but previous said to continue",
		)

		es.pending = message.NotPending
		if es.Current != nil {
			es.objectbody = nil
		} else {
			inslogger.FromContext(ctx).Error(
				"we have object in pending state, but ",
				"with currently executing contract. shouldn't happen",
			)
		}
	} else if es.pending == message.PendingUnknown {
		es.pending = msg.Pending
	}

	//prepare Queue
	if msg.Queue != nil {
		queueFromMessage := make([]ExecutionQueueElement, 0)
		for _, qe := range msg.Queue {
			queueFromMessage = append(
				queueFromMessage,
				ExecutionQueueElement{
					ctx:     qe.Parcel.Context(context.Background()),
					parcel:  qe.Parcel,
					request: qe.Request,
					pulse:   qe.Pulse,
				})
		}
		es.Queue = append(queueFromMessage, es.Queue...)
	}

	es.Unlock()

	err := lr.StartQueueProcessorIfNeeded(ctx, es, msg)
	if err != nil {
		return errors.Wrap(err, "can't start Queue Processor from prepareObjectState")
	}

	return nil
}

func (lr *LogicRunner) executeMethodCall(ctx context.Context, es *ExecutionState, m *message.CallMethod) (core.Reply, error) {
	if es.objectbody == nil {
		objDesc, protoDesc, codeDesc, err := lr.getDescriptorsByObjectRef(ctx, m.ObjectRef)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't get descriptors by object reference")
		}
		es.objectbody = &ObjectBody{
			objDescriptor:   objDesc,
			Object:          objDesc.Memory(),
			Prototype:       protoDesc.HeadRef(),
			CodeMachineType: codeDesc.MachineType(),
			CodeRef:         codeDesc.Ref(),
			Parent:          objDesc.Parent(),
		}
		inslogger.FromContext(ctx).Info("LogicRunner.executeMethodCall starts")
	}

	current := *es.Current
	current.LogicContext.Prototype = es.objectbody.Prototype
	current.LogicContext.Code = es.objectbody.CodeRef
	current.LogicContext.Parent = es.objectbody.Parent
	// it's needed to assure that we call method on ref, that has same prototype as proxy, that we import in contract code
	if !m.ProxyPrototype.IsEmpty() && !m.ProxyPrototype.Equal(*es.objectbody.Prototype) {
		return nil, errors.New("proxy call error: try to call method of prototype as method of another prototype")
	}

	executor, err := lr.GetExecutor(es.objectbody.CodeMachineType)
	if err != nil {
		return nil, es.WrapError(err, "no executor registered")
	}

	newData, result, err := executor.CallMethod(
		ctx, current.LogicContext, *es.objectbody.CodeRef, es.objectbody.Object, m.Method, m.Arguments,
	)
	if err != nil {
		return nil, es.WrapError(err, "executor error")
	}

	am := lr.ArtifactManager
	if es.deactivate {
		_, err := am.DeactivateObject(
			ctx, Ref{}, *current.Request, es.objectbody.objDescriptor,
		)
		if err != nil {
			return nil, es.WrapError(err, "couldn't deactivate object")
		}
	} else {
		od, err := am.UpdateObject(ctx, Ref{}, *current.Request, es.objectbody.objDescriptor, newData)
		if err != nil {
			if strings.Contains(err.Error(), "invalid state record") {
				es.objectbody = nil
			}
			return nil, es.WrapError(err, "couldn't update object")
		}
		es.objectbody.objDescriptor = od
	}
	_, err = am.RegisterResult(ctx, m.ObjectRef, *current.Request, result)
	if err != nil {
		return nil, es.WrapError(err, "couldn't save results")
	}

	es.objectbody.Object = newData

	return &reply.CallMethod{Result: result, Request: *current.Request}, nil
}

func (lr *LogicRunner) getDescriptorsByPrototypeRef(
	ctx context.Context, protoRef Ref,
) (
	core.ObjectDescriptor, core.CodeDescriptor, error,
) {
	protoDesc, err := lr.ArtifactManager.GetObject(ctx, protoRef, nil, false)
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't get prototype descriptor")
	}
	codeRef, err := protoDesc.Code()
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't get code reference")
	}
	// we don't want to record GetCode messages because of cache
	ctx = core.ContextWithMessageBus(ctx, lr.MessageBus)
	codeDesc, err := lr.ArtifactManager.GetCode(ctx, *codeRef)
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't get code descriptor")
	}

	return protoDesc, codeDesc, nil
}

func (lr *LogicRunner) getDescriptorsByObjectRef(
	ctx context.Context, objRef Ref,
) (
	core.ObjectDescriptor, core.ObjectDescriptor, core.CodeDescriptor, error,
) {
	ctx, span := instracer.StartSpan(ctx, "LogicRunner.getDescriptorsByObjectRef")
	defer span.End()

	objDesc, err := lr.ArtifactManager.GetObject(ctx, objRef, nil, false)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "couldn't get object")
	}

	protoRef, err := objDesc.Prototype()
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "couldn't get prototype reference")
	}

	protoDesc, codeDesc, err := lr.getDescriptorsByPrototypeRef(ctx, *protoRef)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "couldn't resolve prototype reference to descriptors")
	}

	return objDesc, protoDesc, codeDesc, nil
}

func (lr *LogicRunner) executeConstructorCall(
	ctx context.Context, es *ExecutionState, m *message.CallConstructor,
) (
	core.Reply, error,
) {
	current := *es.Current
	if current.LogicContext.Caller.IsEmpty() {
		return nil, es.WrapError(nil, "Call constructor from nowhere")
	}

	protoDesc, codeDesc, err := lr.getDescriptorsByPrototypeRef(ctx, m.PrototypeRef)
	if err != nil {
		return nil, es.WrapError(err, "couldn't descriptors")
	}
	current.LogicContext.Prototype = protoDesc.HeadRef()
	current.LogicContext.Code = codeDesc.Ref()

	executor, err := lr.GetExecutor(codeDesc.MachineType())
	if err != nil {
		return nil, es.WrapError(err, "no executer registered")
	}

	newData, err := executor.CallConstructor(ctx, current.LogicContext, *codeDesc.Ref(), m.Name, m.Arguments)
	if err != nil {
		return nil, es.WrapError(err, "executer error")
	}

	switch m.SaveAs {
	case message.Child, message.Delegate:
		_, err = lr.ArtifactManager.ActivateObject(
			ctx,
			Ref{}, *current.Request, m.ParentRef, m.PrototypeRef, m.SaveAs == message.Delegate, newData,
		)
		_, err = lr.ArtifactManager.RegisterResult(ctx, *current.Request, *current.Request, nil)
		if err != nil {
			return nil, es.WrapError(err, "couldn't save results")
		}
		return &reply.CallConstructor{Object: current.Request}, err

	default:
		return nil, es.WrapError(nil, "unsupported type of save object")
	}
}

func (lr *LogicRunner) OnPulse(ctx context.Context, pulse core.Pulse) error {
	lr.stateMutex.Lock()

	ctx, span := instracer.StartSpan(ctx, "pulse.logicrunner")
	defer span.End()

	messages := make([]core.Message, 0)

	ctx, spanStates := instracer.StartSpan(ctx, "pulse.logicrunner processing of states")
	for ref, state := range lr.state {
		meNext, _ := lr.JetCoordinator.IsAuthorized(
			ctx, core.DynamicRoleVirtualExecutor, *ref.Record(), pulse.PulseNumber, lr.JetCoordinator.Me(),
		)
		state.Lock()

		// some old stuff
		state.RefreshConsensus()

		if es := state.ExecutionState; es != nil {
			es.Lock()

			// if we are executor again we just continue working
			// without sending data on next executor (because we are next executor)
			if !meNext {
				sendExecResults := false

				if es.Current != nil {
					es.pending = message.InPending
					sendExecResults = true

					// TODO: this should return delegation token to continue execution of the pending
					messages = append(
						messages,
						&message.StillExecuting{
							Reference: ref,
						},
					)
				} else {
					if es.pending == message.InPending && !es.PendingConfirmed {
						inslogger.FromContext(ctx).Warn(
							"looks like pending executor died, continuing execution",
						)
						es.pending = message.NotPending
						sendExecResults = true
					}

					state.ExecutionState = nil
				}

				queue := es.releaseQueue()
				if len(queue) > 0 || sendExecResults {
					// TODO: we also should send when executed something for validation
					// TODO: now validation is disabled
					caseBind := es.Behaviour.(*ValidationSaver).caseBind
					requests := caseBind.getCaseBindForMessage(ctx)
					messages = append(
						messages,
						//&message.ValidateCaseBind{
						//	RecordRef: ref,
						//	Requests:  requests,
						//	Pulse:     pulse,
						//},
						&message.ExecutorResults{
							RecordRef: ref,
							Pending:   es.pending,
							Requests:  requests,
							Queue:     convertQueueToMessageQueue(queue),
						},
					)
				}
			} else {
				if es.Current != nil {
					// no pending should be as we are executing
					if es.pending == message.InPending {
						inslogger.FromContext(ctx).Warn(
							"we are executing ATM, but ES marked as pending, shouldn't be",
						)
						es.pending = message.NotPending
					}
				} else if es.pending == message.InPending && !es.PendingConfirmed {
					inslogger.FromContext(ctx).Warn(
						"looks like pending executor died, continuing execution",
					)
					es.pending = message.NotPending
					es.objectbody = nil
					go func() {
						err := lr.StartQueueProcessorIfNeeded(ctx, es, nil)
						if err != nil {
							inslogger.FromContext(ctx).Error(
								errors.Wrap(err, "couldn't start queue processor"),
							)

						}
					}()
				}
				es.PendingConfirmed = false
			}

			es.Unlock()
		}

		if state.ExecutionState == nil && state.Validation == nil && state.Consensus == nil {
			delete(lr.state, ref)
		}

		state.Unlock()
	}
	spanStates.End()

	lr.stateMutex.Unlock()

	var sendWg sync.WaitGroup
	ctx, spanMessages := instracer.StartSpan(ctx, "pulse.logicrunner sending messages")
	spanMessages.AddAttributes(trace.StringAttribute("numMessages", strconv.Itoa(len(messages))))

	if len(messages) > 0 {
		sendWg.Add(len(messages))

		for _, msg := range messages {
			go lr.sendOnPulseMessagesAsync(ctx, msg, &sendWg)
		}
	}

	go func() {
		sendWg.Wait()
		spanMessages.End()
	}()

	return nil
}

func (lr *LogicRunner) HandleStillExecutingMessage(
	ctx context.Context, parcel core.Parcel,
) (
	core.Reply, error,
) {
	msg := parcel.Message().(*message.StillExecuting)
	ref := msg.DefaultTarget()
	os := lr.UpsertObjectState(*ref)

	inslogger.FromContext(ctx).Debug("Got information that ", ref, " is still executing")

	os.Lock()
	if os.ExecutionState == nil {
		// we are first, strange, soon ExecuteResults message should come
		os.ExecutionState = &ExecutionState{
			Queue:            make([]ExecutionQueueElement, 0),
			Behaviour:        &ValidationSaver{lr: lr, caseBind: NewCaseBind()},
			pending:          message.InPending,
			PendingConfirmed: true,
		}
	} else {
		es := os.ExecutionState
		es.Lock()
		if es.pending == message.NotPending {
			inslogger.FromContext(ctx).Error(
				"got StillExecuting message, but our state says that it's not in pending",
			)
		} else {
			es.PendingConfirmed = true
		}
		es.Unlock()
	}
	os.Unlock()

	return &reply.OK{}, nil
}

func (lr *LogicRunner) sendOnPulseMessagesAsync(ctx context.Context, msg core.Message, sendWg *sync.WaitGroup) {
	defer sendWg.Done()
	_, err := lr.MessageBus.Send(ctx, msg, nil)
	if err != nil {
		inslogger.FromContext(ctx).Error(errors.Wrap(err, "error while sending validation data on pulse"))
	}
}
func convertQueueToMessageQueue(queue []ExecutionQueueElement) []message.ExecutionQueueElement {
	mq := make([]message.ExecutionQueueElement, 0)
	for _, elem := range queue {
		mq = append(mq, message.ExecutionQueueElement{
			Parcel:  elem.parcel,
			Request: elem.request,
			Pulse:   elem.pulse,
		})
	}

	return mq
}
