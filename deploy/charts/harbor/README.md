# Harbor Supply-Chain Bootstrap

Harbor is the external supply-chain control plane. It is not a Chaimir runtime image, a sandbox capability, or a student-selectable component. Keeping its own containers in `images/` would make the formal Harbor digest lock depend on a Harbor instance that is not running yet.

`bootstrap.lock.json` is the single source of truth for the fixed upstream chart, source commits, runtime base, active chart topology, and admission thresholds. The current topology is deliberately closed: ingress exposure and disabled metrics allow exactly eight runtime containers. The chart preparation path must reject a missing, extra, or tag-only component reference.

The first installation uses an OCI bootstrap bundle that has already passed component Trivy, SBOM, and Cosign bundle verification and has been imported on every target node. Once Harbor is available, normal Chaimir runtime images resume the ordinary Harbor digest-lock admission pipeline. This bootstrap exception does not permit unscanned images, mutable tags, unsigned archives, or a separate local deployment path.
