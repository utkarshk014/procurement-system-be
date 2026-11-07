// Package external is where clients for third-party services live (email,
// SMS, WhatsApp, payment or GST lookup APIs), one sub-package per service,
// mirroring internal/repository: the domain declares the interface, the
// client here implements it, and usecase/index.go wires it in.
//
// Nothing calls out to a third party yet; the database (including the DBA's
// stored procedures) is reached through internal/repository.
package external
