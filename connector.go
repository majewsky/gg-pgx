// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: Apache-2.0

// Package gg_pgx contains integration for using [gg/gsql] (and libraries based on it, such as [Oblast]) with the PostgreSQL driver library [pgx].
//
// [gg/gsql]: https://pkg.go.dev/go.xyrillian.de/gg/gsql
// [Oblast]: https://pkg.go.dev/go.xyrillian.de/oblast
// [pgx]: https://github.com/jackc/pgx
package gg_pgx

// TODO: implement Handle (handle.go is copied from an early prototype in the Oblast benchmark suite and uses the extremely old Wrap() style)
// TODO: implement ConnectionHandle
// TODO: test coverage (via gg/pgruntime)
