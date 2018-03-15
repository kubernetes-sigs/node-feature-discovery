#!/usr/bin/env bash
show_help() { 
cat << EOF
    Usage: ${0##*/} [-a APPLICATION_NAME]
    Runs pods ten times with discovery enabled.

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
echo "Creating pods with node feature discovery enabled."
for i in {1..10}
do
    if [ "$app" == "parsec" ]
    then
        sed -e "s/NUM/$i-with-discovery/" -e "s/APP/demo-1/" demo-pod-with-discovery.yaml.parsec.template > demo-pod-with-discovery.yaml
        kubectl create -f demo-pod-with-discovery.yaml
    else
        sed -e "s/NUM/$i-with-discovery/" -e "s/APP/demo-2/" demo-pod-with-discovery.yaml.cloverleaf.template > demo-pod-with-discovery.yaml
        kubectl create -f demo-pod-with-discovery.yaml
    fi
    echo "WithDiscovery" >> labels-with-discovery-$app.log
done 
echo "Ten pods with node feature discovery enabled started."

rm -f demo-pod-with-discovery.yaml
