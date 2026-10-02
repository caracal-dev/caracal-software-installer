#!/usr/bin/env bash
set -euo pipefail

manifest_path="${HOME}/.local/share/caracal-software-installer/manifests/verdalis-suite.txt"

if [[ -f "${manifest_path}" ]]; then
    while IFS= read -r target; do
        [[ -z "${target}" ]] && continue
        rm -rf "${target}"
    done < "${manifest_path}"
    rm -f "${manifest_path}"
    echo "Verdalis Suite removed using recorded manifest."
    exit 0
fi

names=(ChirpParade CrackleBlaze InsectSwarm NightLife RainyDay RiverFlow ShoreBreak SkyHowl ThunderClap WhooshPact)

for name in "${names[@]}"; do
    rm -rf "${HOME}/.vst3/${name}.vst3"
    rm -f "${HOME}/.clap/${name}.clap"
done

echo "Verdalis Suite removed from user plugin directories."