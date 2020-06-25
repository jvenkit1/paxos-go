# paxos-go
Implementation of Paxos Algorithm

# Brief Description:
Paxos is a protocol for state machine replication in an asynchronous environment that admits crash failures.

## Why Paxos:
Maintaining sync within the environment in case of a multi-user concurrent stream of requests is a challenge and can often lead to inconsistencies in information stored.

## Components of Paxos:
In the Paxos algorithm, we assume a network of processes. Each process plays the following roles:
* Proposer
* Acceptor
* Learner

We also select a leader which acts as a distinguished proposer/learner.

Each process communicates in the form of request-response messages. 

No two proposals are ever issues the same number.



# Implementation Notes:

* Implementing the state machine as a collection of clients that issue commands to a central server. A server plays 3 broad roles - Proposer, Acceptor and Learner.
* Processes interact with each other in the form of messages. A message can be either of Propose, Promise or Accept


# TODO:
* Introduce chaos into the setup. Randomly delete one process and evaluate the effect.  -- This is a test for Byzantine Failure