# HSS Science

**HSS Science** is a unified cloud platform—envisioned as a custom, Google Workspace-like suite of interconnected applications—developed for a specific community under the domain **hss-science.org**.

Centered around a core Identity Provider (IdP) and Single Sign-On (SSO) foundation, this repository (`hss-science`) contains the application source code for the entire platform, including backend microservices, frontend applications, and edge components.

---

## Repository Responsibility

This repository is responsible for:

* Maintaining the **application source code**
* Building applications
* Producing artifacts such as **container images**
* Pushing those artifacts to a container registry

This repository is **not responsible** for:

* Deployment to any environment (e.g., staging, production)
* Infrastructure provisioning
* Runtime orchestration (e.g., Kubernetes configuration)
* Database migrations execution

Database migration **schemes and definitions** may be stored in this repository,
however, the execution of migrations is handled outside of this repository's scope.

In short:

> This repository defines and builds the software.
> It does not deploy or operate the infrastructure.

---

## Note

This is a private, internally developed platform.
It is not an open-source project.

---

## Development Log

### 2026-01-12

* Project initiated.