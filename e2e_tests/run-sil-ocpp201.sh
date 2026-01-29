LD_LIBRARY_PATH=/workspace/dist/lib:$LD_LIBRARY_PATH \
PATH=/workspace/dist/bin:$PATH \
manager \
    --prefix /workspace/dist \
    --conf /home/ubuntu/git/everest-core/config/config-sil-ocpp201-pnc.yaml \
    \
    $@
