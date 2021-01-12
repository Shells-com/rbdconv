# rbdconv

This code allows converting virtual disk data to rbd data, which can then be used with ceph.

For Linux, Shells™ typically requires the following layout:

* 8GB disk image
* DOS or GPT partition table (will be modified with GNU parted so likely anything compatible will work)
* Single /root partition formatted with ext4

The initrd script for Shells™ will perform the following:

* Create swap partition at end of disk
* Resize main partition to fill all available space
* Invoke `/usr/sbin/resize2fs` in mounted root to online resize the actual filesystem

8GB was chosen as the minimum disk size is 10GB, and some room is needed for
swap. Also, most distributions are using up to ~5GB so this should not be an
issue.

Non-linux OSes can also be converted this way, and do not need to be exactly
8GB, however if the image is too large it may not be possible to be used with
plans not offering at least an equal amount of space.

## Usage

Shells™ typically requires images to use rbd+xz format.

	php raw-to-rbd.php diskimage.raw | xz -z -9 -T 16 -v >diskimage.shells

You can check the sha256 sum and compare with the value shown on shells after upload:

	sha256sum -b diskimage.shells

