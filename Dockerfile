FROM busybox

COPY dynamic_admission /usr/local/bin/admission-server

RUN chmod +x /usr/local/bin/admission-server

CMD [ "/usr/local/bin/admission-server" ]
