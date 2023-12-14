##Get image from AWS Public ECR
FROM public.ecr.aws/bitnami/golang:1.16.15 as builder 
## Add current folder to container
ADD . $GOPATH/src/github.com/finnishtransportagency/analytiikka-vesi/container/ais_websocket_reader/
## Set new workfolder
WORKDIR $GOPATH/src/github.com/finnishtransportagency/analytiikka-vesi/container/ais_websocket_reader/
## Get dependencies
RUN go mod download
# RUN go mod download this might be useful later
# Build the binary
RUN go build -o /go/bin/ais_websocket_reader/
##Scratch container as we dont need sutff in build container anymore
## We only want end result of our code to run, so lets for time beign use same image as golang image.. but someone might want to play around with multistage images to find better one
FROM public.ecr.aws/bitnami/golang:1.16.15 
## copy binary file from previous step 
COPY --from=builder /go/bin/ais_websocket_reader/ /go/bin/ais_websocket_reader/
## run binary
ENTRYPOINT ["/go/bin/ais_websocket_reader/"]