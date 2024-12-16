sudo -s

printf "\n172.16.103.96 containerd-control-plane\n" >> /etc/hosts
printf "172.16.103.61 containerd-worker-node\n" >> /etc/hosts

### Install and configure prerequisites https://kubernetes.io/docs/setup/production-environment/container-runtimes/#install-and-configure-prerequisites
cat <<EOF | sudo tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

sudo modprobe overlay
sudo modprobe br_netfilter

# Sysctl params required by setup
cat <<EOF | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

# Apply sysctl params without reboot
sysctl --system
# Prints:
# br_netfilter           32768  0
# bridge                311296  1 br_netfilter

# Verify preconditions were successfully installed
lsmod | grep br_netfilter
# Prints:
# overlay               151552  24

lsmod | grep overlay
# Prints:


sysctl net.bridge.bridge-nf-call-iptables net.bridge.bridge-nf-call-ip6tables net.ipv4.ip_forward
# Prints:
# net.bridge.bridge-nf-call-iptables = 1
# net.bridge.bridge-nf-call-ip6tables = 1
# net.ipv4.ip_forward = 1


### Install Go
wget https://go.dev/dl/go1.22.9.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.22.9.linux-amd64.tar.gz
printf '\nexport PATH=$PATH:/usr/local/go/bin\n' >> /etc/profile
export PATH=$PATH:/usr/local/go/bin
go version
# Prints:
# go version go1.22.9 linux/amd64
rm go1.22.9.linux-amd64.tar.gz


### Install runc
apt update && apt install -y make gcc linux-libc-dev libseccomp-dev pkg-config git
git clone https://github.com/opencontainers/runc && cd runc
git checkout v1.2.0-rc.2
make
make install && cd ..
runc -v
# Prints:
# runc version 1.2.0-rc.2
# commit: v1.2.0-rc.2-0-gf2d2ee5e
# spec: 1.2.0
# go: go1.22.9
# libseccomp: 2.5.3


### Install containerd containing CRIU interface implementation | FOR CRI-O skip this and continue further
git clone https://github.com/adrianreber/containerd.git
git checkout 2024-06-19-restore-create-start
make
make install && cd ..
containerd -v
# Prints:
# containerd github.com/containerd/containerd/v2 v2.0.0-rc.3-134-gb38ddcfe5 b38ddcfe59e10749b55381afaa389cef967e588b
wget https://raw.githubusercontent.com/containerd/containerd/main/containerd.service -P /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now containerd
# Run to check if the service is active:
# systemctl status containerd


mkdir -p /etc/containerd
containerd config default | tee /etc/containerd/config.toml
# Edit:
# [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
#  ...
#  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
#    SystemdCgroup = true
systemctl restart containerd


### Install CRIU
git clone https://github.com/checkpoint-restore/criu && cd criu
git checkout v3.19
apt update && apt install -y libprotobuf-dev libprotobuf-c-dev protobuf-c-compiler protobuf-compiler python3-protobuf \
    pkg-config python3-pip libbsd-dev iproute2 libnftables-dev libcap-dev libnl-3-dev libnet-dev libaio-dev \
    libgnutls28-dev python3-future libdrm-dev asciidoc xmlto --no-install-recommends
make install
criu --version
# Prints:
# Version: 3.19
# GitID: v3.19


### Install Kubernetes v1.30
apt-get update && apt-get install -y apt-transport-https ca-certificates curl gpg

curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.30/deb/Release.key |
    gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg

echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.30/deb/ /" |
    tee /etc/apt/sources.list.d/kubernetes.list

sudo apt-get update && apt-get install -y kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl


# Install CRI-O v1.30 and run it as a systemd service
curl -fsSL https://pkgs.k8s.io/addons:/cri-o:/stable:/v1.30/deb/Release.key |
    gpg --dearmor -o /etc/apt/keyrings/cri-o-apt-keyring.gpg

echo "deb [signed-by=/etc/apt/keyrings/cri-o-apt-keyring.gpg] https://pkgs.k8s.io/addons:/cri-o:/stable:/v1.30/deb/ /" |
    tee /etc/apt/sources.list.d/cri-o.list

sudo apt-get update && apt-get install -y cri-o
systemctl start crio.service

### !!! Older versions of CRI-O require cgroupv1 for checkpoint/restore https://github.com/cri-o/cri-o/issues/6972#issuecomment-1609524097

# Might be useful to reboot but should not be required
# reboot
# sudo -s

### ONLY ON CONTROL NODE .. control plane install:
kubeadm init --pod-network-cidr=192.168.0.0/16 --node-name=control-plane

# Install Flannel CNI as it is more lightweight than Calico https://github.com/flannel-io/flannel/blob/master/README.md
wget https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
# Edit kube-flannel.yml with pod-network-cidr you are using, in this case 192.168.0.0/16
kubectl apply -f kube-flannel.yml

# get worker node commands to run to join additional nodes into cluster
kubeadm token create --print-join-command


## ONLY ON WORKER nodes
# Run the command from the token create output above, example:
# kubeadm join 172.16.101.88:6443 --token vnxnso.abcd --discovery-token-ca-cert-hash sha256:c6b5158ae4e14b114e833afecb0cae3d2142e9e5c60180222cbab83c5b18e912
