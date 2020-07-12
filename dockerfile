FROM centos
COPY ./bin/chest_unix /

ENTRYPOINT ["/chest_unix"]
