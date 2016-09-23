#!/usr/bin/env bash
show_help() { 
cat << EOF
    Usage: ${0##*/} [-a APPLICATION_NAME]
    Aggregate the results from the specified application and plot the result.

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

for i in {1..10}
do
    kubectl logs -f demo-$app-$i-wo-discovery | grep real | cut -f2 | sed -e "s/m/*60+/" -e "s/s//" | bc >> temp.log
done
paste <(cat labels-without-discovery-$app.log) <(cat temp.log) > performance.log
rm -f temp.log labels-without-discovery-$app.log

for i in {1..10}
do
    kubectl logs -f demo-$app-$i-with-discovery | grep real | cut -f2 | sed -e "s/m/*60+/" -e "s/s//" | bc >> temp.log
done
paste <(cat labels-with-discovery-$app.log) <(cat temp.log) >> performance.log
rm -f temp.log labels-with-discovery-$app.log

minimum=$(awk 'min=="" || $2 < min {min=$2} END {print min}' performance.log)
awk -v min=$minimum '{print $1,((($2/min)*100))-100}' performance.log > performance-norm.log
./box-plot.R performance.log performance-comparison-$app.pdf
./box-plot-norm.R performance-norm.log performance-comparison-$app-norm.pdf

./clean-up.sh -a $app
