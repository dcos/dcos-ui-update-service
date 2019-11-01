FROM golang:1.13-alpine AS build-env
ADD . /src
RUN cd /src && go build -o main
RUN mkdir -p /run/dcos


FROM alpine
WORKDIR /app
COPY --from=build-env /src/main /app/
CMD /app/main