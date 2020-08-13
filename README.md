# Brief Description:
Paxos is a protocol for state machine replication in an asynchronous environment that admits crash failures. We have multiple nodes, and our aim is to reach an agreement with all the nodes
regarding accepting particular values. This is a state of consensus and we perform our activity after attaining this state.

## Why Paxos:
Maintaining sync within the environment in case of a multi-user concurrent stream of requests is a challenge and can often lead to inconsistencies in information stored.

## Components of Paxos:
In the Paxos algorithm, we assume a network of processes. There are 3 kinds of roles to be played in a Paxos environment:
* Proposer
* Acceptor
* Learner

Each process communicates in the form of request-response messages and no two proposals are ever issues the same number.

There are 4 types of messages used to communicate :
1. Prepare Message: Signifies a node suggesting a particular value. This message is broadcasted to all nodes.
2. Promise Message: Signifies an agreement between 2 nodes, wherein node B accepts node A's message suggesting a particular value. Signifies intent to recommend this value during the election.
3. Propose Message: Once a node attains majority promises for its proposed value, it sends a Propose message to all the majority nodes, formally proposing the value.
4. Accept Message: 

A paxos cycle begins with each node issuing a request. This forms the basis values for which consensus has to occur. We call this message a 'Prepare Message'. Since we follow a request response model, an acknowledgement to the Prepare message is called as a Promise message.

Once an issued prepared message value attains majority promises, we send a proposal message. Upon verification of this proposal message, we finally accept/learn a particular value.


# Implementation Notes:

* Implementing the state machine as a collection of clients that issue commands to a central server. A server plays 3 broad roles - Proposer, Acceptor and Learner.
* Processes interact with each other in the form of messages. A message can be either of Propose, Promise or Accept.
* Using Go channels to maintain connections with other nodes.


# TODO:
* Introduce chaos into the setup. Randomly delete one process and evaluate the effect.  -- This is a test for Byzantine Failure
* 2 proposer model has a weird deadlock condition. Test further.