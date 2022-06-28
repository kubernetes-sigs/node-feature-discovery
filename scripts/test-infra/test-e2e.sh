#!/bin/bash -e

# Install deps
curl -o $HOME/bin/aws-iam-authenticator --create-dirs -L https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/v0.5.7/aws-iam-authenticator_0.5.7_linux_amd64
chmod a+x $HOME/bin/aws-iam-authenticator
export PATH=$PATH:$HOME/bin


# Configure environment
export KUBECONFIG=`pwd`/kubeconfig
export E2E_TEST_CONFIG=`pwd`/e2e-test-config

echo "$KUBECONFIG_DATA" > "$KUBECONFIG"
echo "$E2E_TEST_CONFIG_DATA" > "$E2E_TEST_CONFIG"


# Wait for the image to be built and published
i=1
while true; do
    if make poll-images; then
        break
    elif [ $i -ge 27 ]; then
        echo "ERROR: too many tries when polling for image"
        exit 1
    fi
    sleep 60

    i=$(( $i + 1 ))
done


# Configure environment and run tests
make e2e-test

