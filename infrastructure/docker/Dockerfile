FROM golang:1.21-alpine AS build
ARG APP_DIR

WORKDIR /server
COPY utils/ /server/utils
WORKDIR /server/utils
RUN go mod download

WORKDIR /server
COPY $APP_DIR/go.mod $APP_DIR/go.sum /server
RUN go mod edit -replace satikin.com/utils=./utils/
RUN go mod download
COPY $APP_DIR/*.go /server
RUN CGO_ENABLED=0 GOOS=linux go build -o dist

FROM scratch
COPY --from=build /server/dist /dist
# USER nonroot:nonroot