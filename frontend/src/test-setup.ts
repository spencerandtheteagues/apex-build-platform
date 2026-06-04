// Ensure React knows we're in a test environment so act() is available.
// This must be set before React is imported.
export {};
(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
