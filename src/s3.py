import subprocess

#TODO
def squash_image(mounted_name, temp_dir):
    print("squashing container image")

    squash_args = ["mksquashfs"]
    args.append(mounted_name)
    args.append(tmp_dir + "/rootfs")

    process = subproccess.run(args,
            stdout=subprocess.PIPE,
            universal_newline=True)
