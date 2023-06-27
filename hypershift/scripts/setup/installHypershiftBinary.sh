#!/bin/bash

oc get ConsoleCLIDownload hypershift-cli-download
if [ $? -ne 0 ]; then
    echo "$(date) failed to get ConsoleCLIDownload hypershift-cli-download"
    exit 1
fi

hypershiftTarGzURL=`oc get ConsoleCLIDownload hypershift-cli-download -o jsonpath='{.spec.links[?(@.text=="Download hypershift CLI for Linux for x86_64")].href}'`
if [ -z "$hypershiftTarGzURL" ]; then
echo "$(date) failed to get Hypershift tar.gz ConsoleCLIDownload hypershift-cli-download"
    exit 1
fi

curl -ko hypershift.tar.gz ${hypershiftTarGzURL}
tar xvzf hypershift.tar.gz
if [ $? -ne 0 ]; then
    echo "$(date) failed to untar hypershift.tar.gz"
    exit 1
fi

chmod +x hypershift
if [ $? -ne 0 ]; then
    echo "$(date) failed to chmod +x hypershift"
    exit 1
fi

mv hypershift /bin
if [ $? -ne 0 ]; then
    echo "$(date) failed to mv hypershift to /bin"
    exit 1
fi

hypershift version


#echo ==== Building latest 4.12 hypershift CLI... ====
#git clone -b release-4.12 -c http.sslVerify=false https://oauth2:${GH_TOKEN}@github.com/openshift/hypershift.git hypershift/
#cd hypershift
#make build
#mv bin/hypershift /usr/local/bin/hypershift
#chmod +x /usr/local/bin/hypershift
#echo
#echo ==== hypershift version ====
#hypershift version
#echo