export type ErrorKind =
  | "EnvironmentNotFound"
  | "EnvironmentAlreadyExists"
  | "AdapterMismatch"
  | "ProfileSetupFailed"
  | "RegistryUnavailable"
  | "CredentialsMissing"
  | "AdapterUnavailable"
  | "CleanupFailed"
  | "InvalidEnvironmentSpec"
  | "InternalInvariantViolation";

export class AgentVenvError extends Error {
  readonly kind: ErrorKind;
  readonly payload: Record<string, unknown>;

  constructor(kind: ErrorKind, message?: string, payload: Record<string, unknown> = {}) {
    super(message ?? kind);
    this.name = "AgentVenvError";
    this.kind = kind;
    this.payload = payload;
  }

  toJSON(): Record<string, unknown> {
    return { kind: this.kind, message: this.message, ...this.payload };
  }
}

function makeSubclass(kind: ErrorKind, name: string) {
  class Sub extends AgentVenvError {
    constructor(message?: string, payload: Record<string, unknown> = {}) {
      super(kind, message, payload);
      this.name = name;
    }
  }
  return Sub;
}

export const EnvironmentNotFoundError = makeSubclass("EnvironmentNotFound", "EnvironmentNotFoundError");
export const EnvironmentAlreadyExistsError = makeSubclass("EnvironmentAlreadyExists", "EnvironmentAlreadyExistsError");
export const AdapterMismatchError = makeSubclass("AdapterMismatch", "AdapterMismatchError");
export const ProfileSetupFailedError = makeSubclass("ProfileSetupFailed", "ProfileSetupFailedError");
export const RegistryUnavailableError = makeSubclass("RegistryUnavailable", "RegistryUnavailableError");
export const CredentialsMissingError = makeSubclass("CredentialsMissing", "CredentialsMissingError");
export const AdapterUnavailableError = makeSubclass("AdapterUnavailable", "AdapterUnavailableError");
export const CleanupFailedError = makeSubclass("CleanupFailed", "CleanupFailedError");
export const InvalidEnvironmentSpecError = makeSubclass("InvalidEnvironmentSpec", "InvalidEnvironmentSpecError");
export const InternalInvariantViolationError = makeSubclass("InternalInvariantViolation", "InternalInvariantViolationError");
