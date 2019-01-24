#!/usr/bin/env bash
set -e

BIN_DIR=bin
TEST_DATA=testdata
INSOLARD=$BIN_DIR/insolard
INSGORUND=$BIN_DIR/insgorund
PULSARD=$BIN_DIR/pulsard
CONTRACT_STORAGE=contractstorage
LEDGER_DIR=data
CONFIGS_DIR=configs
BASE_DIR=scripts/insolard
KEYS_FILE=$BASE_DIR/$CONFIGS_DIR/bootstrap_keys.json
ROOT_MEMBER_KEYS_FILE=$BASE_DIR/$CONFIGS_DIR/root_member_keys.json
NODES_DATA=$BASE_DIR/nodes/
GENESIS_CONFIG=$BASE_DIR/genesis.yaml
GENERATED_CONFIGS_DIR=$BASE_DIR/$CONFIGS_DIR/generated_configs
INSGORUND_PORT_FILE=$BASE_DIR/$CONFIGS_DIR/insgorund_ports.txt

insolar_log_level=Debug
gorund_log_level=$insolar_log_level

NUM_NODES=$(grep "host: " $GENESIS_CONFIG | grep -cv "#" )

for i in `seq 1 $NUM_NODES`
do
    NODES+=($NODES_DATA/$i)
done

DISCOVERY_NODES_KEYS_DIR=$TEST_DATA/scripts/discovery_nodes

kill_port()
{
    port=$1
    pids=$(lsof -i :$port | grep "LISTEN\|UDP" | awk '{print $2}')
    for pid in $pids
    do
        echo "killing pid $pid"
        kill -9 $pid
    done
}

stop_listening()
{
    echo "stop_listening() starts ..."
    stop_insgorund=$1
    ports="$ports 58090" # Pulsar
    ports="$ports 53837" # Genesis
    if [ "$stop_insgorund" == "true" ]
    then
        gorund_ports=
        while read -r line; do

            listen_port=$( echo "$line" | awk '{print $1}' )
            rpc_port=$( echo "$line" | awk '{print $2}' )

            gorund_ports="$gorund_ports $listen_port $rpc_port"

        done < "$INSGORUND_PORT_FILE"

        gorund_ports="$gorund_ports $(echo $(pgrep insgorund ))"

        ports="$ports $gorund_ports"

    fi

    transport_ports=$( grep "host:" $GENESIS_CONFIG | grep -o ":\d\+" | grep -o "\d\+" | tr '\n' ' ' )
    ports="$ports $transport_ports"

    echo "Stop listening..."
    
    for port in $ports
    do
        echo "port: $port"
        kill_port $port
    done
    echo "stop_listening() end."
}

clear_dirs()
{
    echo "clear_dirs() starts ..."
    rm -rfv $CONTRACT_STORAGE/*
    rm -rfv $LEDGER_DIR/*
    rm -rfv $NODES_DATA/*
    rm -rfv $GENERATED_CONFIGS_DIR/*
    echo "clear_dirs() end."
}

create_required_dirs()
{
    echo "create_required_dirs() starts ..."
    mkdir -vp $CONTRACT_STORAGE
    mkdir -vp $LEDGER_DIR
    mkdir -vp $NODES_DATA/certs
    mkdir -vp $GENERATED_CONFIGS_DIR
    touch $INSGORUND_PORT_FILE

    for node in "${NODES[@]}"
    do
        mkdir -vp $node/data
    done

    mkdir -p scripts/insolard/$CONFIGS_DIR

    echo "create_required_dirs() end."
}

generate_insolard_configs()
{
    go run scripts/generate_insolar_configs.go -o $GENERATED_CONFIGS_DIR -p $INSGORUND_PORT_FILE -g $GENESIS_CONFIG -t $BASE_DIR/pulsar_template.yaml
}

prepare()
{
    echo "prepare() starts ..."
    stop_listening $run_insgorund
    clear_dirs
    create_required_dirs
    echo "prepare() end."
}

build_binaries()
{
    make build
}

rebuild_binaries()
{
    make clean
    build_binaries
}

generate_bootstrap_keys()
{
    echo "generate_bootstrap_keys() starts ..."
	bin/insolar -c gen_keys > $KEYS_FILE
	echo "generate_bootstrap_keys() end."
}

generate_root_member_keys()
{
    echo "generate_root_member_keys() starts ..."
	bin/insolar -c gen_keys > $ROOT_MEMBER_KEYS_FILE
	echo "generate_root_member_keys() end."
}

generate_discovery_nodes_keys()
{
    echo "generate_discovery_nodes_keys() starts ..."
    for node in "${NODES[@]}"
    do
        bin/insolar -c gen_keys > $node/keys.json
    done
    echo "generate_discovery_nodes_keys() end."
}

check_working_dir()
{
    echo "check_working_dir() starts ..."
    if ! pwd | grep -q "src/github.com/insolar/insolar$"
    then
        echo "Run me from insolar root"
        exit 1
    fi
    echo "check_working_dir() end."
}

usage()
{
    echo "usage: $0 [options]"
    echo "possible options: "
    echo -e "\t-h - show help"
    echo -e "\t-n - don't run insgorund"
    echo -e "\t-g - preventively generate initial ledger"
    echo -e "\t-l - clear all and exit"
}

process_input_params()
{
    OPTIND=1
    while getopts "h?ngl" opt; do
        case "$opt" in
        h|\?)
            usage
            exit 0
            ;;
        n)
            run_insgorund=false
            ;;
        g)
            genesis
            ;;
        l)
            prepare
            exit 0
            ;;
        esac
    done
}

launch_insgorund()
{
    host=127.0.0.1
    metrics_port=28223
    while read -r line; do

        metrics_port=$((metrics_port + 20))
        listen_port=$( echo "$line" | awk '{print $1}' )
        rpc_port=$( echo "$line" | awk '{print $2}' )

        $INSGORUND -l $host:$listen_port --rpc $host:$rpc_port --log-level=$gorund_log_level --metrics :$metrics_port &

    done < "$INSGORUND_PORT_FILE"
}

copy_data()
{
    echo "copy_data() starts ..."
    for node in "${NODES[@]}"
    do
        cp -v $LEDGER_DIR/* $node/data
    done
    echo "copy_data() end."
}

copy_certs()
{
    echo "copy_certs() starts ..."
    i=0
    for node in "${NODES[@]}"
    do
        i=$((i + 1))
        cp -v $NODES_DATA/certs/discovery_cert_$i.json $node/cert.json
    done
    echo "copy_certs() end."
}

genesis()
{
    prepare
    build_binaries
    generate_bootstrap_keys
    generate_root_member_keys
    generate_discovery_nodes_keys
    generate_insolard_configs

    printf "start genesis ... \n"
    $INSOLARD --config $BASE_DIR/insolar.yaml --genesis $GENESIS_CONFIG --keyout $NODES_DATA/certs
    printf "genesis is done\n"

    copy_data
    copy_certs


    if which jq ; then
        NL=$BASE_DIR/loglinks
        mkdir  $NL || \
        rm -f $NL/*.log
        for node in "${NODES[@]}" ; do
            ref=`jq -r '.reference' $node/cert.json`
            [[ $ref =~ .+\. ]]
            ln -s `pwd`/$node/output.log $NL/${BASH_REMATCH[0]}log
        done
    else
        echo "no jq =("
    fi
}

trap 'stop_listening true' INT TERM EXIT

run_insgorund=true
check_working_dir
process_input_params $@

printf "start pulsar ... \n"
$PULSARD -c $GENERATED_CONFIGS_DIR/pulsar.yaml --trace &> $NODES_DATA/pulsar_output.log &

if [ "$run_insgorund" == "true" ]
then
    printf "start insgorund ... \n"
    launch_insgorund
else
    echo "INSGORUND IS NOT LAUNCHED"
fi

printf "start nodes ... \n"

i=0
for node in "${NODES[@]}"
do
    i=$((i + 1))
    if [ "$i" -eq "$NUM_NODES" ]
    then
        echo "NODE $i STARTED in foreground"
        INSOLAR_LOG_LEVEL=$insolar_log_level $INSOLARD --config $GENERATED_CONFIGS_DIR/insolar_$i.yaml --trace &> $node/output.log
        break
    fi
    INSOLAR_LOG_LEVEL=$insolar_log_level $INSOLARD --config $GENERATED_CONFIGS_DIR/insolar_$i.yaml --trace &> $node/output.log &
    echo "NODE $i STARTED in background"
done

echo "FINISHING ..."
