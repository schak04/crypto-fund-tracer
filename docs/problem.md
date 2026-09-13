# Problem Understanding

## Background

Cryptocurrency is increasingly used in cyber fraud cases such as investment scams, task-based fraud, ransomware, phishing, sextortion and other forms of cyber-enabled financial crime.

In many of these cases, the victim is instructed to send cryptocurrency to a wallet address controlled or used by the fraudster.

The victim can report this wallet address to the authorities as part of a cybercrime complaint.

## The Investigation Problem

A reported wallet address is only the starting point.

Investigators need to determine what happened to the funds after they reached that address.

For example:

```
Victim
  |
  | Cryptocurrency
  v
Suspect Wallet
  |
  v
Intermediate Wallet (could be multiple)
  |
  v
Exchange / VASP
```

The funds may pass through several wallets, services, or even different blockchain networks before reaching an identifiable destination.

Manually following these transactions can require significant time and blockchain expertise.

## What This Project Tries to Do

The proposed system starts with a victim-reported suspect wallet address and automatically analyses its blockchain activity.

The system should attempt to:

1. retrieve relevant transaction data
2. trace the movement of funds
3. identify important intermediary wallets
4. determine whether funds reach known exchanges or VASPs
5. identify relevant fund-flow patterns
6. present the findings as useful investigative intelligence

The important distinction is that a blockchain address does not automatically reveal the identity of its owner.

Therefore, tracing transactions and identifying the real-world entity associated with an address are separate problems.

## Example

Suppose a victim reports:

```
Wallet A
```

The system may discover:

```
Wallet A
    |
    v
Wallet B
    |
    v
Wallet C
    |
    v
Known Exchange Wallet
```

The system should present this flow, along with relevant transaction information and the identified exchange where sufficient evidence is available.

## Initial Scope

Has been written about [here](./mvp.md).

## Expected Outcome

The final system should help an investigator go from:

```
Suspect wallet address
        |
        v
Traced fund flow
        +
Relevant transaction evidence
        +
Known exchange/VASP attribution where possible
        +
Investigation-friendly result
```

The purpose is to reduce the amount of manual work required to perform the initial stages of cryptocurrency fund tracing.
