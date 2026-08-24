# tool/workspace

This image is the platform-private student workspace sidecar. It provides the
restricted file, archive, and terminal helper used by runtime composition
snapshots. It is not a chain runtime, judge, or selectable teacher tool.

The image is pulled only by immutable digest after its manifest, self-test,
signature, and vulnerability gates pass.
