FROM	golang:latest	AS	build-stage

WORKDIR	/app
COPY	go.mod	go.sum	./
RUN	go mod download
COPY	*.go	./
RUN	CGO_ENABLED=0 GOOS=linux go build -o /goprohtds
FROM	alpine:latest
WORKDIR	/
COPY	--from=build-stage /goprohtds	/goprohtds
ENTRYPOINT	["/goprohtds"]
