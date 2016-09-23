#!/usr/bin/env bash
show_help() { 
cat << EOF
    Usage: ${0##*/} [-a APPLICATION_NAME]
    Clean-up pods with and without discovery enabled for the specified application. 

    -a APPLICATION_NAME     clean-up the pods with APPLICATION_NAME application.
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
for i in {1..10}
do 
    kubectl delete po demo-$app-$i-wo-discovery
done

for i in {1..10}
do 
    kubectl delete po demo-$app-$i-with-discovery
done
