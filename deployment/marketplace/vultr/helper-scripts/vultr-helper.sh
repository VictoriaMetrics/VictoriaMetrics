#!/bin/bash

###################################################################
## Vultr Marketplace Helper Functions

inherit_errexit()
{
	if ! shopt -sq inherit_errexit > /dev/null 2>&1; then
		echo "Unable to enable inherit_errexit"
	fi
}

error_detect_on()
{
	set -euo pipefail
	inherit_errexit
}

error_detect_off()
{
	set +euo pipefail
	inherit_errexit
}

enable_verbose_commands()
{
	set -x
}

disable_verbose_commands()
{
	set +x
}

get_metadata_item()
{
	local item_value
	item_value="$(curl --fail --silent --header "Metadata-Token: vultr" "http://169.254.169.254/${1:-}")"

	echo "${item_value}"
}

get_hostname()
{
	get_metadata_item "latest/meta-data/hostname"
}

get_userdata()
{
	get_metadata_item "latest/user-data"
}

get_sshkeys()
{
	get_metadata_item "current/ssh-keys"
}

# shellcheck disable=SC2034
get_var()
{
	local var_name="${1:-}" var_path="${2:-}" var_val
	var_val="$(get_metadata_item "${var_path:-"v1/internal/app-${var_name}"}" 2> /dev/null)"

	eval "${var_name}=\${var_val}"
}

# shellcheck disable=SC2034
get_ip()
{
	local ip_var="${1:-}" ip_val
	ip_val="$(get_var "${ip_var}" "latest/meta-data/public-ipv4")"

	eval "${ip_var}=\${ip_val}"
}

wait_on_apt_lock()
{
	until ! lsof -t /var/cache/apt/archives/lock /var/lib/apt/lists/lock /var/lib/dpkg/lock > /dev/null 2>&1; do
		echo "Waiting 3 for apt lock currently held by another process."
		sleep 3
	done
}

apt_safe()
{
	wait_on_apt_lock
	apt install -y "$@"
}

apt_update_safe()
{
	wait_on_apt_lock
	apt update -y
}

apt_upgrade_safe()
{
	wait_on_apt_lock
	DEBIAN_FRONTEND=noninteractive apt upgrade -y
}

apt_remove_safe()
{
	wait_on_apt_lock
	apt remove -y --auto-remove "$@"
}

apt_clean_safe()
{
	wait_on_apt_lock
	apt autoremove -y

	wait_on_apt_lock
	apt autoclean -y
}

update_and_clean_packages()
{
	# RHEL/CentOS
	if [[ -f /etc/redhat-release ]]; then
		yum update -y
		yum clean all
	# Ubuntu / Debian
	elif grep -qs "debian" /etc/os-release 2> /dev/null; then
		apt_update_safe
		apt_upgrade_safe
		apt_clean_safe
	fi
}

set_vultr_kernel_option()
{
	# RHEL/CentOS
	if [[ -f /etc/redhat-release ]]; then
		/sbin/grubby --update-kernel=ALL --args vultr
	# Ubuntu / Debian
	elif grep -qs "debian" /etc/os-release 2> /dev/null; then
		sed -i -e "/^GRUB_CMDLINE_LINUX_DEFAULT=/ s/\"$/ vultr\"/" /etc/default/grub
		update-grub
	fi
}

install_cloud_init()
{
	local cloudinit_exe=""
	if cloudinit_exe="$(command -v cloud-init 2> /dev/null)" && [[ -x "${cloudinit_exe}" ]]; then
		echo "cloud-init is already installed."
		return
	fi

	local release_version="${1:-"latest"}"
	if [[ "${release_version}" != "latest" && "${release_version}" != "nightly" ]]; then
		echo "${release_version} is an invalid release option. Allowed: latest, nightly"
		exit 255
	fi

	# Lets remove all traces of previously installed cloud-init
	# Ubuntu installs have proven problematic with their left over
	# configs for the installer in recent versions
	cleanup_cloudinit

	update_and_clean_packages

	local build_type
	local package_ext

	[[ -e /etc/os-release ]] && . /etc/os-release
	case "${ID:-}" in
		debian)
			build_type="debian"
			package_ext="deb"
			;;
		fedora)
			build_type="rhel"
			package_ext="rpm"
			;;
		ubuntu)
			build_type="universal"
			package_ext="deb"
			;;
		*)
			case "${ID_LIKE:-}" in
				*rhel*)
					build_type="rhel"
					package_ext="rpm"
					;;
				*)
					echo "Unable to determine OS. Please install from source!"
					exit 255
					;;
			esac
			;;
	esac

	local cloud_init_package="cloud-init_${build_type}_${release_version}.${package_ext}"
	wget -O "/tmp/${cloud_init_package}" "https://ewr1.vultrobjects.com/cloud_init_beta/${cloud_init_package}"

	case "${package_ext}" in
		rpm)
			yum install -y "/tmp/${cloud_init_package}"
			;;
		deb)
			apt_safe "/tmp/${cloud_init_package}"
			;;
		*)
			echo "Unable to determine package installation method."
			exit 255
			;;
	esac

	rm -f "/tmp/${cloud_init_package}"
}

cleanup_cloudinit()
{
	rm -rf \
		/etc/cloud \
		/etc/systemd/system/cloud-init.target.wants/* \
		/lib/systemd/system/cloud* \
		/run/cloud-init \
		/usr/bin/cloud* \
		/usr/lib/cloud* \
		/usr/local/bin/cloud* \
		/usr/src/cloud* \
		/var/log/cloud*
}

clean_tmp()
{
	mkdir -p /tmp
	chmod 1777 /tmp
	rm -rf /tmp/* /var/tmp/*
}

clean_keys()
{
	rm -f /root/.ssh/authorized_keys /etc/ssh/*key*
	touch /etc/ssh/revoked_keys
	chmod 600 /etc/ssh/revoked_keys
}

clean_logs()
{
	find /var/log -mtime -1 -type f -exec truncate -s 0 {} \;
	rm -rf \
		/var/log/*.[0-9] \
		/var/log/*.gz \
		/var/log/*.log \
		/var/log/lastlog \
		/var/log/wtmp

	: > /var/log/auth.log
}

clean_history()
{
	history -c
	: > /root/.bash_history
	unset HISTFILE
}

clean_mloc()
{
	/usr/bin/updatedb || true
}

clean_random()
{
	rm -f /var/lib/systemd/random-seed
}

clean_machine_id()
{
	[[ -e /etc/machine-id ]] && : > /etc/machine-id
	[[ -e /var/lib/dbus/machine-id ]] && : > /var/lib/dbus/machine-id
}

clean_free_space()
{
	dd if=/dev/zero of=/zerofile || true
	sync
	rm -f /zerofile
	sync
}

trim_ssd()
{
	fstrim / || true
}

cleanup_marketplace_scripts()
{
	rm -f /root/*.sh
}

disable_network_manager()
{
	## Disable NetworkManager, replace with network-scripts
	systemctl disable --now NetworkManager
	sed -i \
		-e 's/^ONBOOT.*/ONBOOT=yes/g' \
		-e 's/^NM_CONTROLLED.*/NM_CONTROLLED=no/g' /etc/sysconfig/network-scripts/ifcfg-*
	yum install -y network-scripts
}

clean_system()
{

	update_and_clean_packages
	set_vultr_kernel_option
	clean_tmp
	clean_keys
	clean_logs
	clean_history
	clean_random
	clean_machine_id
	clean_mloc
	clean_free_space
	trim_ssd

	cleanup_marketplace_scripts
}
