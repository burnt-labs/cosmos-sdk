# Security patch provenance

This repo-local fork is versioned `2.0.3-burnt.0` and is built from the upstream
`image-size` 2.0.2 source. It is kept repo-local because no patched upstream
release is available for the two denial-of-service advisories affecting that
baseline:

- `GHSA-w3rx-r6r6-pgpr`: reject zero-length and truncated ICNS entries.
- `GHSA-5p2g-fcmc-qvqq`: reject ISO BMFF boxes smaller than their eight-byte
  header, covering the JXL and HEIF parser loops.

The package preserves the 2.x root and `fromFile` exports required by
Docusaurus. Its checked-in `dist` files were generated from the upstream source
with `npm run build`; `../../scripts/check-image-size-security.js` exercises both
guards and the `fromFile` compatibility contract.
