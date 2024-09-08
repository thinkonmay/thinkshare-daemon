FROM golang as build
WORKDIR /artifact
WORKDIR /source
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN go build -o /artifact/daemon ./service/linux


FROM ubuntu 
RUN apt-get update -y && \
    apt install -y virt-manager  \
    net-tools \
    curl \
    vim \
    neofetch

WORKDIR /
COPY --from=build /artifact/daemon .
ENTRYPOINT [ "/daemon" ]