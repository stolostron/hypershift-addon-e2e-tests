#!/bin/bash

#########################################
#  POPULATE THESE WITH ENV VARS        #
# export HC_CLI_OS=     # valid options: linux (DEFAULT) / darwin / windows
# export HC_CLI_ARCH=   # valid options: amd64 (DEFAULT) / arm64/ ppc64 / ppc64le / s390x
#########################################
HCP_BINARY_NAME=${HC_BINARY:-hcp}
MCE_NS=$(oc get "$(oc get multiclusterengines -oname)" -ojsonpath="{.spec.targetNamespace}")
HC_CLI_OS=${HC_CLI_OS:-linux}       # valid options: linux / darwin / windows
HC_CLI_ARCH=${HC_CLI_ARCH:-amd64}   # valid options: amd64 / arm64/ ppc64 / ppc64le / s390x

echo "$(date) MCE_NS = ${MCE_NS}"
echo "$(date) HC_CLI_OS = ${HC_CLI_OS}" 
echo "$(date) HC_CLI_ARCH = ${HC_CLI_ARCH}" 

echo "$(date) Curl, extract, and move to CLI to PATH"
# The cli-download server now serves flat filenames (hcp-<os>-<arch>.tar.gz) at the
# route root, not the old nested /<os>/<arch>/hcp.tar.gz path. Build the URL to match.
HCP_CLI_HOST=$(oc get routes ${HCP_BINARY_NAME}-cli-download -n ${MCE_NS} -o jsonpath="{.spec.host}")
HCP_CLI_URL="https://${HCP_CLI_HOST}/${HCP_BINARY_NAME}-${HC_CLI_OS}-${HC_CLI_ARCH}.tar.gz"
echo "curl -kfLo ${HCP_BINARY_NAME}.tar.gz ${HCP_CLI_URL}"
# -f makes curl fail (non-zero exit) on HTTP errors like 404 instead of saving the
# error page and failing later at the untar step with a confusing gzip error.
curl -kfLo ${HCP_BINARY_NAME}.tar.gz "${HCP_CLI_URL}"
if [ $? -ne 0 ]; then
    echo "$(date) failed to curl ${HCP_BINARY_NAME}.tar.gz from ${HCP_CLI_URL}"
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
