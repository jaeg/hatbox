FROM centos
COPY ./bin/chest_unix /
RUN mkdir  /contents

ENTRYPOINT ["/chest_unix"]
