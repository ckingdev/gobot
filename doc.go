// Package gobot provides a golang framework for bots running on the euphoria.io
// chat service.
//
// Bots can have multiple Rooms, which can be added, removed, started, and
// stopped dynamically. The Handler interface allows the user to easily write
// their own code to extend the framework that can reply to incoming packets
// and also run in the background, with the ability to manipulate the room it is
// attached to.
//
// A bolt database and a scope.Context are exposed by the room to be used as the
// user sees fit. The framework itself uses the Context but not the database.
// Therefore, one should be careful with scope.Context.Waitgroup() - do not Wait
// unless you are familiar with the internals of the package.
//
// The bolt database provides a persistent key-value store on disk and the
// scope.Context provides an in-memory key-value store. The framework does not
// use either of these stores, so the user may use them without fear of
// collisions.
//
// A small example of a handler is included in the handlers package. It simply
// replies to a message of "!ping" with "pong!". The program in the sample
// package uses this to create a simple, functioning bot with the framework.
package gobot
