FROM centos
COPY ./bin/hatbox_unix /
RUN mkdir  /contents

ENTRYPOINT ["/hatbox_unix"]
