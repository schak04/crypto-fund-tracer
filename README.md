# Crypto Fund Tracer

A blockchain analytics system for tracing cryptocurrency funds from victim-reported suspect wallet addresses and identifying their eventual destination, particularly cryptocurrency exchanges and VASPs.

## The Problem

> For details: [`problem.md`](docs/problem.md)

In many cyber fraud cases, victims are instructed to send cryptocurrency to wallet addresses controlled or used by fraudsters.

Investigators can inspect the blockchain to follow these funds, but doing this manually can be time-consuming, especially when funds pass through multiple wallets or services.

The goal of this project is to automate part of that process.

Given a suspect wallet address, the system should:

- collect relevant blockchain transaction data
- trace the movement of funds
- identify relevant intermediary wallets and destinations
- identify known exchanges or VASPs where possible
- present the resulting fund flow in a form useful to investigators

The initial version will focus on a small, well-defined scope and will be expanded only after the MVP is complete. It has been written about in detail here: [`mvp.md`](docs/mvp.md).

# System Design

## High-Level Design

- [`design.md`](./docs/hld/design.md)
