import subprocess
import boto3
import os
import tempfile
# local imports
from utils import cmd, get_os
import logging

def publish(cname, args):

    layer_name = args['name']
    publish_tags = args['publish_tags']
    if type(publish_tags) is not list:
        publish_tags = [publish_tags]
    if 'credentials' in args:
        credentials = args['credentials']
    parent = args['parent']
    
    if args['publish_local']:
        for tag in publish_tags:
            cmd(["buildah","commit", cname, layer_name+':'+tag], stderr_handler=logging.warn)

    if args['publish_s3']:
        print("pushing to s3")
        s3_prefix = args['s3_prefix']
        s3_bucket = args['s3_bucket']
        for tag in publish_tags:
            s3_push(cname, layer_name, credentials, tag, s3_prefix, s3_bucket)

    if args['publish_registry']:
        registry_opts = args['registry_opts_push']
        publish_dest = args['publish_registry']
        for tag in publish_tags:
            cmd(["buildah", "commit", cname, layer_name+':'+tag], stderr_handler=logging.warn)
            registry_push(layer_name, registry_opts, tag, publish_dest)

    # Clean up
    cmd(["buildah", "rm", cname], stderr_handler=logging.warn)
    if not args['publish_local'] and args['publish_registry']:
        for tag in publish_tags:
            cmd(["buildah","rmi", layer_name+':'+tag], stderr_handler=logging.warn)
    if not parent == "scratch":
        cmd(["buildah", "rmi", parent], stderr_handler=logging.warn)

def push_file(fname, kname, s3, bucket_name):
    print("Pushing " + fname + " as " + kname + " to " + bucket_name)

    bucket = s3.Bucket(bucket_name)
    bucket.upload_file(Filename=fname,Key=kname)

def squash_image(mname, tmpdir):
    print("squashing container image")
    args = ["mksquashfs"]
    args.append(mname)
    args.append(tmpdir + "/rootfs")

    process = subprocess.run(args,
            stdout=subprocess.PIPE,
            universal_newlines=True)
    # if verbose:
    #     print(process.stdout)

def s3_push(cname, layer_name, credentials, publish_tags, s3_prefix, s3_bucket):

    def buildah_handler(line):
            out.append(line)
    out = []
    cmd(["buildah", "mount", cname],stdout_handler = buildah_handler)
    mdir = out[0]
   
    print(mdir)

    # Get s3 resource set
    s3 = boto3.resource('s3',
                    endpoint_url=credentials['endpoint_url'],
                    aws_access_key_id=credentials['access_key'],
                    aws_secret_access_key=credentials['secret_key'],
                    verify=False, use_ssl=False)

    kver=os.listdir(mdir+'/lib/modules/')[0]
    if os.path.isfile(mdir+'/boot/initramfs-'+kver+'.img'):
        initrd='initramfs-'+kver+'.img'
    elif os.path.isfile(mdir+'/boot/initrd-'+kver):
        initrd='initrd-'+kver
    vmlinuz='vmlinuz-'+kver

    with tempfile.TemporaryDirectory() as tmpdir:
        squash_image(mdir, tmpdir)
        image_name = s3_prefix+get_os(mdir)+'-'+layer_name+'-'+publish_tags
        print("Image Name: " + image_name)
        print("initramfs: " + initrd )
        print("vmlinuz: " + vmlinuz )
        push_file(mdir+'/boot/'+initrd, 'efi-images/' + s3_prefix + initrd, s3, s3_bucket)
        push_file(mdir+'/boot/'+vmlinuz, 'efi-images/' + s3_prefix + vmlinuz, s3, s3_bucket)
        push_file(tmpdir + '/rootfs', image_name, s3, s3_bucket)

def registry_push(layer_name, registry_opts, publish_tags, registry_endpoint):

    image_name = layer_name+':'+publish_tags
    print("pushing layer " + layer_name + " to " + registry_endpoint +'/'+image_name)
    args = registry_opts + [image_name, registry_endpoint +'/'+image_name]
    cmd(["buildah", "push"] + args, stderr_handler=logging.warn)