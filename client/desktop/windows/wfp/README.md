# Windows WFP Helper

This directory contains the first WFP helper scaffold for the desktop client:

- `novpn_wfp_helper.cpp`

Current scope:

- creates a provider and sublayer for NoVPN desktop filters
- can install app-scoped permit filters for IPv4 and IPv6 connect layers
- can clear filters created by the helper

Current status:

- source only
- not compiled or packaged into the Windows desktop build by default
- intended for follow-up work around deeper Windows split-tunnel behavior
