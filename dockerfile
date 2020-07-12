FROM centos
COPY ./bin/wart_unix /

ENTRYPOINT ["/wart_unix"]
