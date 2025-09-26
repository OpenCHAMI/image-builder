FROM registry.suse.com/suse/sle15:15.5
# Copy zypp repo info for builder host container
# permissions are needed for rootlest builds
RUN  touch /run/zypp.pid && \
     chmod 0666 /run/zypp.pid

#COPY ansible/ /etc/ansible
#RUN chmod +r /etc/ansible/ansible.cfg

# Put certs into place
#COPY certs/platform* /etc/pki/trust/anchors/
#COPY certs/platform* /etc/cray/ca/certificate_authority.crt
#COPY certs/lanl* /usr/local/share/ca-certificates/lanl/
#COPY certs/LANL_WIN_OLT_Root_CA.pem /etc/pki/trust/anchors/
#RUN  chmod 0444 /etc/pki/trust/anchors/* && \
#     chmod 0444 /etc/cray/ca/certificate_authority.crt && \
#     chmod 0444 /usr/local/share/ca-certificates/lanl/* && \
#     update-ca-certificates

# remove stupid package and file that tries to mount things that aren't there
RUN rpm -e container-suseconnect
RUN rm -f /etc/containers/mounts.conf

# Copy in imgconfig bins
COPY src/* /usr/local/bin/
RUN chmod 0755 /usr/local/bin/*

# install some useful packages
RUN zypper clean --all
RUN zypper -n --gpg-auto-import-keys ref -f -d
RUN zypper -n update
RUN zypper -n --no-gpg-checks install \
	bash \
	vim \
	git \
        curl \
	wget \
	buildah \
	python311 \
	python311-pip \
        fuse-overlayfs \
	tar \
        squashfs \
        libcap-progs \
        cni \
        cni-plugins \
        rsync \
        unzip

# Python3.9 fixes
#RUN rm /usr/bin/python3 && \
RUN ln -s /usr/bin/python3.11 /usr/bin/python3

# Remove stupid SUSE file that throws warnings all over the place
RUN rm -f /etc/containers/mounts.conf

# install ansible-bender
#RUN pip3.11 --proxy=http://192.12.184.19:8080 install ansible==4.10.0 ansible-base==2.10.17 ansible-bender boto3 dnspython requests jinja2_ansible_filters
RUN pip3.11 --proxy=http://192.12.184.19:8080 install ansible ansible-bender boto3 dnspython requests jinja2_ansible_filters

RUN echo "%_netsharedpath /dev" >> /etc/rpm/macros

# allow fuse mount
COPY containers/storage.conf /etc/containers/storage.conf
RUN chmod 0644 /etc/containers/storage.conf

# Allow non-root to run buildah commands
RUN setcap cap_setuid=ep "$(command -v newuidmap)" && \
    setcap cap_setgid=ep "$(command -v newgidmap)" &&\
    chmod 0755 "$(command -v newuidmap)" && \
    chmod 0755 "$(command -v newgidmap)" && \
    echo "builder:2000:50000" > /etc/subuid && \
    echo "builder:2000:50000" > /etc/subgid

# Create local user for rootless image builds
RUN useradd --uid 1002 builder && \
    mkdir -p /home/builder && \
    chown -R builder /home/builder

# Make localuser the default user when running container
USER builder
WORKDIR /home/builder

ENV BUILDAH_ISOLATION=chroot

ENTRYPOINT ["/usr/bin/buildah", "unshare"]
