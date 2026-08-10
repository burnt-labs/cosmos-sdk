# Resolve Dependabot vulnerabilities

## Goal

Resolve every open Dependabot alert on the active Burnt Labs Cosmos SDK fork while preserving its multi-module Go build and documentation behavior.

## Plan

1. Consolidate the existing Dependabot Go and documentation updates onto one branch.
2. Remove the remaining unpatched `image-size` exposure with a reviewed repo-local package containing the upstream malformed-input fixes and a regression check.
3. Rebuild the documentation lockfile and update every Go module sum consistently.
4. Verify a frozen documentation install, zero npm audit findings, the image parser regressions, Go module integrity, and focused builds/tests for changed modules.
5. Publish one natural security PR and verify its mergeability and hosted checks.

## Safety boundary

Do not alter Cosmos SDK runtime behavior or suppress advisories. The vendored parser must remain API-compatible with the locked `image-size` version and must fail closed on malformed ICNS and zero-length ISO BMFF boxes.
