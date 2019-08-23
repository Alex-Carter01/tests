#!/bin/bash
# Copyright (c) 2018-2019 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Take the data found in subdirectories of the metrics 'results' directory,
# and turn them into a PDF report. Use a Dockerfile containing all the tooling
# and scripts we need to do that.

set -e

SCRIPT_PATH=$(dirname "$(readlink -f "$0")")
source "${SCRIPT_PATH}/../lib/common.bash"

IMAGE="${IMAGE:-metrics-report}"
DOCKERFILE="${SCRIPT_PATH}/report_dockerfile/Dockerfile"

HOSTINPUTDIR="${SCRIPT_PATH}/../results"
RENVFILE="${HOSTINPUTDIR}/Env.R"
HOSTOUTPUTDIR="${SCRIPT_PATH}/output"

GUESTINPUTDIR="/inputdir/"
GUESTOUTPUTDIR="/outputdir/"

# If in debugging mode, we also map in the scripts dir so you can
# dynamically edit and re-load them at the R prompt
HOSTSCRIPTDIR="${SCRIPT_PATH}/report_dockerfile"
GUESTSCRIPTDIR="/scripts/"

setup() {
	echo "Checking subdirectories"
	check_subdir="$(ls -dx ${HOSTINPUTDIR}/*/ 2> /dev/null | wc -l)"
	if [ $check_subdir -eq 0 ]; then
		die "No subdirs in [${HOSTINPUTDIR}] to read results from."
	fi

	echo "Checking Dockerfile"
	check_dockerfiles_images "$IMAGE" "$DOCKERFILE"

	mkdir -p "$HOSTOUTPUTDIR" && true

	echo "inputdir=\"${GUESTINPUTDIR}\"" > ${RENVFILE}
	echo "outputdir=\"${GUESTOUTPUTDIR}\"" >> ${RENVFILE}

	# A bit of a hack to get an R syntax'd list of dirs to process
	# Also, need it as not host-side dir path - so short relative names
	resultdirs="$(cd ${HOSTINPUTDIR}; ls -dx */)"
	resultdirslist=$(echo ${resultdirs} | sed 's/ \+/", "/g')
	echo "resultdirs=c(" >> ${RENVFILE}
	echo "	\"${resultdirslist}\"" >> ${RENVFILE}
	echo ")" >> ${RENVFILE}
}

run() {
	docker run -ti --rm -v ${HOSTINPUTDIR}:${GUESTINPUTDIR} -v ${HOSTOUTPUTDIR}:${GUESTOUTPUTDIR} ${extra_volumes} ${IMAGE} ${extra_command}
	ls -la ${HOSTOUTPUTDIR}/*
}

help() {
	usage=$(cat << EOF
Usage: $0 [-h] [options]
   Description:
        This script generates a metrics report document
        from the results directory one level up in the
        directory tree (../results).
   Options:
        -d,         Run in debug (interactive) mode
        -h,         Print this help
EOF
)
	echo "$usage"
}

main() {

	local OPTIND
	while getopts "d" opt;do
		case ${opt} in
		d)
			# In debug mode, run a shell instead of the default report generation
			extra_command="bash"
			extra_volumes="-v ${HOSTSCRIPTDIR}:${GUESTSCRIPTDIR}"
			;;
		?)
		    # parse failure
		    help
		    die "Failed to parse arguments"
		    ;;
		esac
	done
	shift $((OPTIND-1))

	setup
	run
}

main "$@"
