FROM golang:1.16.3-alpine
WORKDIR /app/
# COPY go.mod .
# COPY go.sum .
# RUN go mod download
COPY . .

RUN go build -o executable .

CMD [ "./executable" ]


