#!/bin/bash

#########################################
#  POPULATE THESE WITH ENV VARS        #
# export HC_CLI_OS=     # valid options: linux (DEFAULT) / darwin / windows
# export HC_CLI_ARCH=   # valid options: amd64 (DEFAULT) / arm64/ ppc64 / ppc64le / s390x
#########################################
HCP_BINARY_NAME="hypershift"
MCE_NS=$(oc get "$(oc get multiclusterengines -oname)" -ojsonpath="{.spec.targetNamespace}")
echo "$(date) MCE_NS = ${MCE_NS}"
echo "$(date) HC_CLI_OS = ${HC_CLI_OS:-linux}" # valid options: linux / darwin / windows
echo "$(date) HC_CLI_ARCH = ${HC_CLI_ARCH:-amd64}" # valid options: amd64 / arm64/ ppc64 / ppc64le / s390x

echo "$(date) Curl, extract, and move to CLI to PATH"
curl -ko ${HCP_BINARY_NAME}.tar.gz "https://$(oc get routes ${HCP_BINARY_NAME}-cli-download -n ${MCE_NS} -ojsonpath="{.spec.host}")/${HC_CLI_OS}/${HC_CLI_ARCH}/${HCP_BINARY_NAME}.tar.gz"
if [ $? -ne 0 ]; then
    echo "$(date) failed to curl ${HCP_BINARY_NAME}.tar.gz"
    exit 1
fi

tar xvzf ${HCP_BINARY_NAME}.tar.gz #-C $HOME
if [ $? -ne 0 ]; then
    echo "$(date) failed to untar ${HCP_BINARY_NAME}.tar.gz"
    exit 1
fi

chmod +x ${HCP_BINARY_NAME}
if [ $? -ne 0 ]; then
    echo "$(date) failed to chmod +x ${HCP_BINARY_NAME}"
    exit 1
fi

if [ "$HC_CLI_OS" == "linux" ]; then
    echo "$(date) moving ${HCP_BINARY_NAME} to /bin"
    mv ${HCP_BINARY_NAME} /bin
else
    echo "$(date) moving ${HCP_BINARY_NAME} to /usr/local/bin/."
    mv ${HCP_BINARY_NAME} /usr/local/bin/.
fi

if [ $? -ne 0 ]; then
    echo "$(date) failed to move ${HCP_BINARY_NAME} binary to main path"
    exit 1
fi

echo "$(date) ${HCP_BINARY_NAME} CLI version installed:"
if $(${HCP_BINARY_NAME} version | grep -q 'openshift/hypershift'); then
  date
  ${HCP_BINARY_NAME} version
  echo "$(date) You are ready to provision a hosted plane cluster!"
else
  echo "$(date) ERROR: ${HCP_BINARY_NAME} CLI failed!"
fi

# clean-up
rm -f ${HCP_BINARY_NAME}.tar.gz