# Feature Specification: Multiple Cloud Desktops per Developer

**Feature Branch**: `feature/multi-ec2-instance`

**Created**: 2026-10-21

**Status**: Draft

**Input**: User description: "We're adding a new feature in the clouddesktop where engineers can have multiple and manage remote instances"

## Context

Today each developer can have exactly one cloud desktop. The tool records a single environment per developer, every command acts on that environment implicitly, and setup refuses to run while an environment exists. A developer who needs a second machine (a larger one for a heavy build next to their everyday one, a clean one to reproduce a bug, or a throwaway one to try a risky tooling change) must either destroy their only environment and lose its data, or build one by hand outside the tool. Hand-built machines skip the tool's ownership tagging, access model, idle auto-stop and cleanup.

Several resources are currently one-per-developer rather than one-per-environment: the connection alias, the uploaded access key, the personal object-storage bucket, and the lookup that recovers an environment when local settings are lost. Destroying the environment today removes all of them. Once a developer has more than one environment, removing a resource that another environment still depends on would break that environment or delete its data.

## Clarifications

### Session 2026-10-21

- Q: What is the per-developer limit on environments? → A: The tool enforces no limit.
- Q: How is the target chosen when a developer with several environments does not name one? → A: The tool uses a default environment that the developer chooses and can switch. Destroy never falls back to the default: it requires the environment name whenever the developer has more than one environment.
- Q: What is the personal storage bucket's scope and lifecycle? → A: One bucket per developer, shared by all of that developer's environments. Destroy never deletes it; only an explicit bucket command does.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create an additional environment alongside my existing one (Priority: P1)

As a developer who already has a cloud desktop, I create an additional named environment with its own machine size and disk size. My existing environment is untouched, both exist at the same time, and I can connect to either.

**Why this priority**: This is the core ask. Every other story depends on a developer being able to own more than one environment.

**Independent Test**: Starting from one existing environment, create a second named environment with a different machine size. Verify that both exist, that the first environment's state, data, size and connection settings are unchanged, and that the developer can connect to each.

**Acceptance Scenarios**:

1. **Given** a developer with one environment, **When** they create a new environment with a name and a machine size, **Then** the new environment is provisioned and the existing environment's state, data, size and connection settings are unchanged. The developer's personal bucket becomes available on the new environment the first time they start it after its first-boot setup completes.
2. **Given** a developer who already has an environment named `build`, **When** they try to create another environment named `build`, **Then** the tool refuses, says the name is taken, and creates nothing.
3. **Given** a developer supplies a name that breaks the naming rules, **When** they try to create the environment, **Then** the tool rejects the name before creating any resources and states the rules.
4. **Given** a developer with a default environment, **When** they create an additional environment, **Then** the default environment does not change.

---

### User Story 2 - See all my environments at a glance (Priority: P1)

As a developer with several environments, I run one command and see every environment I own, with its name, state, machine size and disk size, and which one is my default.

**Why this priority**: A developer cannot manage what they cannot see. A forgotten running environment costs money every hour, and a forgotten stopped environment still pays for its disk.

**Independent Test**: Create environments in different states (one running, one stopped). Run the list command and verify that each environment appears with the correct name, state, machine size and disk size, that the default is marked, and that no environment was started or changed.

**Acceptance Scenarios**:

1. **Given** a developer with multiple environments in different states, **When** they list environments, **Then** each one is shown with its name, current state, machine size and disk size, and the default environment is marked.
2. **Given** an environment was removed outside the tool, **When** the developer lists environments, **Then** that environment no longer appears. If it was the default, the list says the default no longer exists and explains how to set a new one.
3. **Given** a developer with no environments, **When** they list environments, **Then** the tool says there are none, shows whether the developer's personal bucket still exists, and explains how to create an environment.

---

### User Story 3 - Target a specific environment, or rely on my default (Priority: P1)

As a developer with several environments, I can start, stop, connect to, check status of, resize and destroy any one of them by name. I can also choose a default environment so everyday commands work without typing a name. Each command affects only the environment it targets and tells me which one that is.

**Why this priority**: Without per-environment targeting, a second environment can be created but not operated, and commands risk acting on the wrong machine.

**Independent Test**: With two running environments, stop one by name and verify that the other is still running. Set the other as the default, run status without a name, and verify that it reports on the default and names it. Destroy one by name and verify that the other is still reachable and its data intact.

**Acceptance Scenarios**:

1. **Given** environments `main` and `build` are both running, **When** the developer stops `build`, **Then** `build` stops and `main` keeps running.
2. **Given** environments `main` and `build`, **When** the developer destroys `build` and confirms, **Then** `build` and its disk are permanently removed, `main` is unaffected and reachable, and the personal bucket and its contents are intact.
3. **Given** a developer names an environment that does not exist, **When** they run any command, **Then** the command fails, lists the valid environment names, and changes nothing.
4. **Given** a developer with exactly one environment, **When** they run any command without naming an environment, **Then** the command acts on that environment exactly as it does today.
5. **Given** a developer with environments `main` and `build` and `build` set as the default, **When** they run a command without naming an environment, **Then** the command acts on `build` and states that it is acting on `build` before it does anything.
6. **Given** a developer with more than one environment, **When** they run destroy without naming an environment, **Then** the tool refuses, lists the environment names, and destroys nothing, even when a default is set.
7. **Given** a developer with more than one environment and no default set, **When** they run a command without naming an environment, **Then** the command fails, lists the environment names, and explains how to set a default.
8. **Given** a developer with environments `main` and `build`, **When** they set `main` as the default, **Then** later commands without a name act on `main`, and the list marks `main` as the default.

---

### User Story 4 - Existing users keep working without re-provisioning (Priority: P1)

As a developer who set up my cloud desktop with the previous version of the tool, I upgrade the tool and keep working. My existing environment becomes my first named environment and my default automatically, and I connect to it the same way I always have.

**Why this priority**: Every current user is in this situation. A migration that forces re-provisioning or loses data blocks adoption of the whole feature.

**Independent Test**: Take a single-environment setup created by the previous version. Install the new version and run status, connect and start. Verify that the environment was adopted under the name `main` and set as the default, that the existing connection alias still works, and that no cloud resources were created, stopped or destroyed during adoption.

**Acceptance Scenarios**:

1. **Given** an environment created by the previous version, **When** the developer runs any command with the new version, **Then** the environment is adopted as an environment named `main`, set as the default, and the command completes without extra setup.
2. **Given** an adopted environment, **When** the developer connects using the connection alias they used before, **Then** the connection reaches the same environment.
3. **Given** adoption cannot complete, **When** the tool reports the failure, **Then** the developer's previous local settings are intact and the developer can retry.
4. **Given** an adopted environment, **When** the developer inspects their personal bucket, **Then** it is the same bucket with the same contents as before the upgrade.

---

### User Story 5 - Recover access to all my environments on a new machine (Priority: P2)

As a developer whose local settings were lost, or who moved to a new laptop, I run setup and the tool finds every environment I already own and restores my access to each one under its original name, without creating new environments.

**Why this priority**: Recovery exists today for a single environment. It must keep working for several environments, but it is needed less often than the day-to-day stories.

**Independent Test**: With two existing environments, delete the local settings, run setup, and verify that both environments are listed under their original names and can be connected to, that setup asked which one to make the default, and that no new environments were created.

**Acceptance Scenarios**:

1. **Given** a developer with two environments and no local settings, **When** they run setup, **Then** both environments are discovered, listed under their original names, and reachable.
2. **Given** setup discovers more than one environment, **When** discovery finishes, **Then** setup asks the developer which environment to make the default.
3. **Given** other developers have environments in the same account, **When** a developer runs discovery, **Then** only environments owned by that developer are adopted.

---

### User Story 6 - Stop all my environments at once (Priority: P3)

As a developer at the end of the day, I run one command to stop every running environment I own.

**Why this priority**: This is a cost-control convenience. Idle auto-stop already acts as a safety net, so this is useful but not required for the feature to work.

**Independent Test**: With three environments, two of them running, run stop-all and verify that both running environments stop and that the outcome is reported for each one.

**Acceptance Scenarios**:

1. **Given** three environments, two of them running, **When** the developer runs stop-all, **Then** both running environments stop and the tool reports the outcome for each environment.
2. **Given** one environment fails to stop, **When** stop-all runs, **Then** the remaining environments are still stopped and the failure is reported with the affected environment's name.

---

### Edge Cases

- The developer runs two commands against different environments at the same time from two terminals, for example creating `build` while stopping `main`. Both must succeed, and neither command may lose the other's update to local settings.
- The developer changes the default in one terminal while working in another. Later commands without a name in the other terminal act on the new default. Each command names its target before acting (FR-023), so the change is visible.
- Creating an environment fails partway (capacity, quota or permission errors). Resources created for that environment must be removed, the developer's other environments must be unaffected, and the failed environment must not be listed as healthy.
- An environment's name would produce a connection alias that already exists in the developer's connection settings but was not created by the tool. The tool must not overwrite or remove entries it does not manage. It must refuse the name and explain why.
- An environment is deleted outside the tool, for example from the cloud console. It no longer appears in the list, and commands that name it fail with the list of valid environment names. If it was the default, commands that rely on the default say so and explain how to set a new one.
- The default environment is destroyed. The tool must not pick a new default on its own. If more than one environment remains, commands without a name fail until the developer sets a new default. If exactly one remains, commands act on it under the single-environment rule (FR-006).
- The developer destroys their last remaining environment. The personal bucket and its contents are kept (FR-012). The confirmation says so and explains how to delete the bucket.
- The developer deletes the personal bucket while one or more environments exist. Every environment that uses it is affected. The tool must list them, and must delete the bucket only when the developer has explicitly confirmed; running without confirmation shows the list and changes nothing.
- The developer resizes one environment. No other environment's size, state or connection settings may change.
- The cloud account reaches an account-wide limit on machines or capacity. The tool must name the limit and what the developer can do about it (FR-004), and must not leave a partially created environment behind.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Developers MUST be able to create additional environments, each with a developer-chosen name that is unique among that developer's environments.
- **FR-002**: Environment names MUST follow the same rules as developer names (lowercase letters, digits and hyphens, starting and ending with a letter or digit) and MUST be at most 20 characters.
- **FR-003**: Each environment MUST have its own machine size and disk size, chosen at creation from the existing supported options, and changeable later without affecting any other environment.
- **FR-004**: The tool MUST NOT limit the number of environments a developer can own. When an account-wide capacity limit blocks creating or starting an environment, the tool MUST name the limit that was reached and what the developer can do about it (for example, stop environments they aren't using, or request a higher limit).
- **FR-005**: Every command that acts on an environment today (start or provision, stop, status, connect, resize machine, resize storage, destroy) MUST accept an environment name and MUST act only on that environment.
- **FR-006**: When a developer has exactly one environment, commands without a name MUST act on it, as they do today. When a developer with more than one environment runs a command without naming one, the tool MUST act on the developer's default environment. If no default is set, the command MUST fail, list the environment names, and explain how to set a default.
- **FR-007**: The tool MUST provide a command that lists every environment the developer owns, with name, current state, machine size, disk size, and which environment is the default. The list MUST also show whether the developer's personal bucket exists, including when the developer has no environments. Listing MUST NOT start or change any environment.
- **FR-008**: The list MUST reflect the actual state of each environment in the cloud. An environment removed outside the tool MUST no longer appear. If the developer's default environment no longer exists, the list and every command that relies on the default MUST say so and explain how to set a new default.
- **FR-009**: Each environment MUST be reachable through its own connection alias derived from its name, usable from terminal sessions, remote editors and file-sync tools without manual edits. Creating or destroying an environment MUST NOT change another environment's alias or any connection entry the tool does not manage.
- **FR-010**: Destroying an environment MUST permanently remove only that environment and the resources it alone uses (machine and disk). Developer-level resources MUST be kept while any of the developer's other environments uses them, and the personal bucket MUST always be kept (FR-012).
- **FR-011**: The destroy confirmation MUST name the environment being destroyed, state that the developer's other environments are not affected, and state that the personal bucket and its contents are kept, with the command to delete it.
- **FR-012**: Each developer MUST have one personal object-storage bucket, shared by and available on every one of the developer's environments. Creating an environment MUST create the bucket first if the developer has none. The bucket MUST become available on an environment the first time the developer starts it after its first-boot setup has completed, and MUST be checked, and repaired if needed, on every later start. Destroying environments, including the developer's last one, MUST NOT delete the bucket or its contents. The bucket MUST be deleted only by an explicit bucket command that requires confirmation and lists every environment that currently uses it.
- **FR-013**: The first time the new version runs against a setup created by the previous version, the existing environment MUST be adopted as an environment named `main` and set as the default, without its machine or disk being stopped, restarted, re-created or modified. Adding a metadata label that records the environment's name is allowed. The connection alias the developer used before MUST keep reaching it, and its existing bucket MUST remain the developer's personal bucket.
- **FR-014**: If adoption fails, the developer's previous local settings MUST remain intact and usable for a retry, and the tool MUST report what failed.
- **FR-015**: When local settings are missing, setup MUST discover every existing environment owned by the developer and restore access to each one under its original name, without creating environments. If more than one environment is found, setup MUST ask the developer to choose the default. Environments owned by other developers MUST never be adopted.
- **FR-016**: Commands running at the same time against different environments MUST NOT corrupt or lose each other's updates to local settings.
- **FR-017**: If creating an environment fails, every resource created for it MUST be removed, and the developer's other environments MUST be unaffected.
- **FR-018**: Each environment MUST auto-stop when idle, independently of the developer's other environments, under the existing idle rules.
- **FR-019**: The tool MUST provide a single command that stops all of the developer's running environments. It MUST report the outcome for each environment and MUST continue past individual failures.
- **FR-020**: Each environment MUST be identifiable by owner and environment name in the organization's cost and inventory reporting.
- **FR-021**: Creating additional environments MUST NOT require re-running setup. Setup MUST continue to refuse to change the developer name or access key while any environment exists.
- **FR-022**: Developers MUST be able to set and change their default environment. The first environment a developer creates MUST become the default. Creating additional environments MUST NOT change the default. When the default environment is destroyed, the tool MUST NOT choose a new default on its own.
- **FR-023**: Every command that acts on an environment MUST state the target environment's name before taking any action, including in every confirmation prompt.
- **FR-024**: Destroy MUST NOT fall back to the default environment. When the developer has more than one environment, destroy MUST require the environment name explicitly.

### Key Entities

- **Developer**: The person who owns environments. Attributes: developer name (the unique identity), cloud account profile, region, access key, and defaults for new environments. Owns zero or more environments and one personal bucket.
- **Environment**: One cloud desktop. Attributes: name (unique per developer), machine size, disk size, current state, connection alias, owner, creation time, and whether it is the developer's default.
- **Default Environment**: The environment that commands act on when the developer has more than one environment and names none. At most one per developer. It is chosen by the developer, recorded in local settings, and may be unset.
- **Personal Storage Bucket**: Durable object storage. There is one per developer, shared by and available on all of that developer's environments. It outlives environments and is deleted only on explicit request.
- **Local Settings**: The record on the developer's laptop of developer-level settings and the default environment. It holds no list of environments; which environments exist is always read from the cloud. Losing it loses only the default, which setup asks for again (FR-015).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer with an existing environment can create and connect to an additional environment with one create command and one connect command, and no manual edits to any configuration file.
- **SC-002**: In 100% of tested operations on one environment (start, stop, resize, destroy), every other environment's state, data, size and connection alias is unchanged, verified by before-and-after comparison.
- **SC-003**: 100% of existing single-environment users can use every command after upgrading the tool, with zero re-provisioning, zero data loss, and no change to how they connect.
- **SC-004**: A developer sees the state of all of their environments with one command, and the result appears within 10 seconds for a developer with up to 10 environments.
- **SC-005**: After losing local settings, a developer regains access to all of their existing environments with one setup run, and no environments are re-created.
- **SC-006**: In 100% of tested destroy operations, including destroying the developer's last environment, the personal bucket's contents are intact afterwards.
- **SC-007**: In 100% of tested commands that act on an environment, the target environment's name is shown before any action is taken.

## Assumptions

- All of a developer's environments use the same cloud account profile and region chosen at setup. Running environments across several regions is out of scope; only one region is enabled today.
- Environments are personal. Sharing an environment with another developer is out of scope.
- Renaming an environment is out of scope for this release. A developer can create a new environment and destroy the old one instead.
- Creating an environment as a copy of another (cloning disk contents) is out of scope.
- One access key per developer is used for all of that developer's environments.
- The existing environment is adopted under the name `main` rather than `default`, so that the name is not confused with the default-environment concept. This is a chosen default, not a stated requirement.
- The 20-character name limit keeps names derived from the developer and environment names within cloud platform length limits. It is a chosen default, not a stated requirement.
- The default environment is recorded only in the developer's local settings. It is not shared between a developer's laptops, and setup asks for it again after recovery.
- Each new environment receives the same first-boot tooling setup and network isolation as environments do today. The shared infrastructure (network, access role, security boundaries) needs no changes.
- Developers' existing cloud permissions already allow them to create more than one environment. This must be confirmed during planning.
- Each environment is billed separately, and a stopped environment still pays for its disk. The tool does not limit the number of environments or enforce a budget. Idle auto-stop and cost reporting (FR-020) are the controls.
- All developers share the cloud account's capacity quota for running machines. Without a per-developer limit, one developer running many large environments can prevent others from starting theirs. Idle auto-stop is the main mitigation.
- Keeping the bucket when an environment is destroyed changes today's behavior, where destroy also deletes the bucket. User-facing documentation must be updated to match.
- The tool does not coordinate simultaneous writes to the same file in the personal bucket from two environments.
