FROM centos:7

RUN yum install -y git

ENTRYPOINT ["helmboot"]

COPY ./build/linux/helmboot /usr/bin/helmboot