#!/usr/bin/env bash

set -eo pipefail

this=`basename $0`
this_dir=`dirname $0`

show_help() {
cat << EOF
    Usage: $this [-a APPLICATION_NAME]
    Runs ten pods without discovery enabled with the specified application.

    -a APPLICATION_NAME     run the pods with APPLICATION_NAME application.
                            APPLICATION_NAME can be one of parsec or cloverleaf.
EOF
}

if [ $# -eq 0 ]
then
    show_help
    exit 1
fi

app="parsec"

OPTIND=1
options="ha:"
while getopts $options option
do
    case $option in
        a)
            if [ "$OPTARG" == "parsec" ] || [ "$OPTARG" == "cloverleaf" ]
            then
                app=$OPTARG
            else
                echo "Invalid application name."
                show_help
                exit 0
            fi
            ;;
        h)
            show_help
            exit 0
            ;;
        '?')
            show_help
            exit 1
            ;;
   esac
done

echo "Using application name = $app."
echo "Creating pods without node feature discovery enabled."
for i in {1..10}
do
    if [ "$app" == "parsec" ]
    then
        sed -e "s/NUM/$i-wo-discovery/" -e "s/IMG/demo-1/" -e "s/APP/$app/" \
            "$this_dir/demo-pod-without-discovery.yaml.template" | kubectl create -f -
    else
        sed -e "s/NUM/$i-wo-discovery/" -e "s/IMG/demo-2/" -e "s/APP/$app/" \
            "$this_dir/demo-pod-without-discovery.yaml.template" | kubectl create -f -
    fi
    echo "WithoutDiscovery" >> labels-without-discovery-$app.log
done
echo "Ten pods without node feature discovery started."
