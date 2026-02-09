# Creating a Microservice

Using a [coding agent](../blocks/coding-agents.md) is the best way of working with microservices, including the creation of new ones.

## Using a Coding Agent

#### Step 1: Initialize the Coding Agent

Start your coding agent, e.g.:

```shell
claude
```

#### Step 2: Create a New Microservice

Use a prompt similar to the following to create a new microservice.

> HEY CLAUDE...
>
> Create a new "stripe" microservice that will process credit card payments via Stripe. Place it under the @financial/ directory. Set its hostname to "stripe.financial.ex".

#### Step 3: Implement the Business Logic

Use prompts such as the following to add one [feature](../blocks/features.md) at a time to the microservice:

> HEY CLAUDE...
>
> Add a config property that will hold the Stripe API key. Make it secret.

> HEY CLAUDE...
>
> Create a functional endpoint "CreateIntent" that accepts a credit card number, email, and a dollar amount as arguments, and calls the Stripe API to create an intent. If successful, it should return the transaction ID.
