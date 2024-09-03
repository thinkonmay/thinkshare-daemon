FROM golang as build
WORKDIR /artifact
WORKDIR /source
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN go build -o /artifact/daemon ./service/linux


FROM ubuntu 
RUN apt-get update -y
RUN apt install -y virt-manager  \
    net-tools \
    vim \
    neofetch

WORKDIR /
COPY --from=build /artifact/daemon .
ENTRYPOINT [ "/daemon" ]