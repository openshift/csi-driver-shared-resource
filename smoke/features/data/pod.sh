#!/bin/bash
set -x

export READ_VALUE=true

create_pod(){
  oc create -f - << EOF
  kind: Pod
  apiVersion: v1
  metadata:
    name: my-csi-app-check
  spec:
    containers:
      - name: my-frontend
        image: busybox
        volumeMounts:
        - mountPath: "/data"
          name: my-csi-volume
        command: [ "/bin/sh" ]
        args: [ "-c", "while true; do ls -la /data; touch /data/bar; ls -la /data; echo sleeping; sleep 10;done" ]
    serviceAccountName: default
    volumes:
      - name: my-csi-volume
        csi:
          readOnly: $1
          driver: csi.sharedresource.openshift.io
          volumeAttributes:
            $2: $3
EOF
}

if [[ "${1}" == *"false"* ]]; then
  export READ_VALUE=false
fi

if [[ "${1}" == *"sharedconfigmap"* ]]; then
  # create pods with volumeAttribute sharedConfigMap
  create_pod $READ_VALUE sharedConfigMap my-shared-config
else
  # create pods with volumeAttribute sharedSecret
  create_pod $READ_VALUE sharedSecret my-shared-secret
fi