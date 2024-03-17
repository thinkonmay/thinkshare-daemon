package libvirt


const (
	libvirtVM = 
`
space: ""
local: domain
type: kvm
name: vm5
memory:
    unit: KiB
    value: 16777216
currentmemory:
    unit: KiB
    value: 16777216
vcpu:
    placement: static
    value: 16
os:
    boot:
        dev: hd
    type:
        arch: x86_64
        machine: pc-i440fx-focal
        value: hvm
    smbios:
        mode: host
features:
    acpi: {}
    apic: {}
    vmport:
        state: "off"
    kvm:
        hidden: 
            state: on
cpu:
    mode: host-passthrough
    check: none
    topology:
        socket: 1
        cores: 16
        thread: 1
    feature:
        policy: disable
        name: hypervisor
clock:
    offset: localtime
    timers:
        - name: hpet
          tickpolicy: null
          present: no
        - name: hypervclock
          tickpolicy: null
          present: yes
        - name: pit
          tickpolicy: delay
          present: null
        - name: rtc
          tickpolicy: catchup
          present: null
onreboot:
    value: restart
onpoweroff:
    value: destroy
oncrash:
    value: destroy
pm:
    suspendtomem:
        enable: "no"
    suspendtodisk:
        enable: "no"
emulator:
    value: /usr/bin/qemu-system-x86_64
disk:
    - driver:
        name: qemu
        type: qcow2
      source:
        file: /disk/2TB1/AtlatOS-copy.qcow2
        index: 1
      target:
        dev: hda
        bus: ide
      address:
        type: drive
        controller: 0
        bus: 0
        target: 0
        unit: 0
      type: file
      device: disk
controllers:
    - type: usb
      index: 0
      model: ich9-ehci1
      master: null
      address:
        type: pci
        domain: "0x0000"
        bus: "0x00"
        slot: "0x05"
        function: "0x7"
        multifunction: null
    - type: usb
      index: 0
      model: ich9-uhci1
      master:
        startport: 0
      address:
        type: pci
        domain: "0x0000"
        bus: "0x00"
        slot: "0x05"
        function: "0x0"
        multifunction: "on"
    - type: usb
      index: 0
      model: ich9-uhci2
      master:
        startport: 2
      address:
        type: pci
        domain: "0x0000"
        bus: "0x00"
        slot: "0x05"
        function: "0x1"
        multifunction: null
    - type: usb
      index: 0
      model: ich9-uhci3
      master:
        startport: 4
      address:
        type: pci
        domain: "0x0000"
        bus: "0x00"
        slot: "0x05"
        function: "0x2"
        multifunction: null
    - type: pci
      index: 0
      model: pci-root
      master: null
      address: null
    - type: ide
      index: 0
      model: null
      master: null
      address:
        type: pci
        domain: "0x0000"
        bus: "0x00"
        slot: "0x01"
        function: "0x1"
        multifunction: null
    - type: virtio-serial
      index: 0
      model: null
      master: null
      address:
        type: pci
        domain: "0x0000"
        bus: "0x00"
        slot: "0x06"
        function: "0x0"
        multifunction: null
interfaces:
    - type: network
      source:
        network: network
      model:
        type: virtio
input: 
    - type: tablet
      bus: usb
graphic:
    type: spice
    autoport: "yes"
    listen:
        type: address
    image:
        compression: "off"
video:
    model:
        ram: 65536
        vram: 65536
        vgamem: 16384
        heads: 1
        type: qxl
        primary: "yes"
    address:
        type: pci
        domain: "0x0000"
        bus: "0x00"
        slot: "0x02"
        function: "0x0"
hostdevs:
    - mode: subsystem
      type: pci
      managed: "yes"
      sourceaddress:
        domain: "0x0000"
        bus: "0x03"
        slot: "0x00"
        function: "0x0"
      address:
        type: pci
        domain: "0x0000"
        bus: "0x00"
        slot: "0x07"
        function: "0x0"
    - mode: subsystem
      type: pci
      managed: "yes"
      sourceaddress:
        domain: "0x0000"
        bus: "0x03"
        slot: "0x00"
        function: "0x1"
      address:
        type: pci
        domain: "0x0000"
        bus: "0x00"
        slot: "0x08"
        function: "0x0"
memballoon:
    model: virtio
    address:
        type: pci
        domain: "0x0000"
        bus: "0x00"
        slot: "0x09"
        function: "0x0"
`
)
