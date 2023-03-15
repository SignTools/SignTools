# How does this all work?

This project is not one simple program. It is a combination of a web service and a builder, which work together to achieve signing and sideloading.

Below is a rough [sequence diagram](https://en.wikipedia.org/wiki/Sequence_diagram) of how the entire process works. If you haven't read a diagram like this before, it essentialy describes interactions between different parties. In this case, we have four parties: the User, Web Service, Builder, and Apple. Start reading the diagram from the top and make your way to the bottom. Each vertical line is a party, while each horizontal line is an interaction. The big rectangle labeled `alt - if using a developer account` will only be executed if you are signing with a developer account. Otherwise, it is skipped.

```mermaid
sequenceDiagram
    User ->>Web Service: Upload unsigned app
    Web Service->>Web Service: Save app and generate sign job
    Web Service->>Builder: Trigger (activate)
        Builder->>Web Service: Retrieve last sign job
        Web Service->>Builder: 
        note over Web Service, Builder: The sign job is an archive of <br> files such as the signing certificate, <br> developer account (if used), <br> and unsigned app
    alt if using a developer account
    rect rgb(0, 0, 255, .1)
        Builder ->>Apple: Start log in to account
        Apple->>User: Send 2FA code
        note over Web Service, Builder: 2FA = Two-factor authentication
        User->>Web Service: Submit 2FA code
        Builder->>Web Service: Retrieve 2FA code
        Web Service->>Builder: 
        Builder->>Apple: Finish log in to account
    end
    end
    Builder ->>Builder: Sign the app
    Builder ->>Web Service: Upload signed app
    User->>Web Service: Install signed app
    Web Service->>User: 
    User->>User: Done
```

