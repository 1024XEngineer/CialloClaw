/* eslint-disable react-refresh/only-export-components */

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  type PropsWithChildren,
} from "react";

type DashboardEscapeHandlerRegistration = {
  enabled: boolean;
  handleEscape: () => void;
  priority: number;
};

type DashboardEscapeCoordinatorValue = {
  consumeEscape: () => boolean;
  registerHandler: (registration: DashboardEscapeHandlerRegistration) => () => void;
};

const DashboardEscapeCoordinatorContext = createContext<DashboardEscapeCoordinatorValue | null>(null);

/**
 * Hosts route-local Escape handlers so dashboard overlays unwind one layer at a
 * time before the desktop window fallback is allowed to close the shell.
 */
export function DashboardEscapeCoordinatorProvider({ children }: PropsWithChildren) {
  const handlersRef = useRef(new Map<number, DashboardEscapeHandlerRegistration>());
  const nextHandlerIdRef = useRef(0);

  const registerHandler = useCallback((registration: DashboardEscapeHandlerRegistration) => {
    const handlerId = nextHandlerIdRef.current++;
    handlersRef.current.set(handlerId, registration);

    return () => {
      handlersRef.current.delete(handlerId);
    };
  }, []);

  const consumeEscape = useCallback(() => {
    const nextHandler = [...handlersRef.current.values()]
      .sort((left, right) => right.priority - left.priority)
      .find((registration) => registration.enabled);

    if (!nextHandler) {
      return false;
    }

    nextHandler.handleEscape();
    return true;
  }, []);

  const value = useMemo<DashboardEscapeCoordinatorValue>(
    () => ({
      consumeEscape,
      registerHandler,
    }),
    [consumeEscape, registerHandler],
  );

  return <DashboardEscapeCoordinatorContext.Provider value={value}>{children}</DashboardEscapeCoordinatorContext.Provider>;
}

/**
 * Returns the route-level Escape coordinator so the dashboard root can decide
 * whether to close local overlays, navigate home, or fall through to window close.
 */
export function useDashboardEscapeCoordinator() {
  const context = useContext(DashboardEscapeCoordinatorContext);

  if (!context) {
    throw new Error("Dashboard Escape coordinator is unavailable outside the dashboard provider.");
  }

  return context;
}

type UseDashboardEscapeHandlerOptions = {
  enabled: boolean;
  handleEscape: () => void;
  priority: number;
};

/**
 * Registers a local dashboard Escape layer without installing another global
 * window listener, keeping all route fallback rules centralized in DashboardRoot.
 */
export function useDashboardEscapeHandler({ enabled, handleEscape, priority }: UseDashboardEscapeHandlerOptions) {
  const coordinator = useDashboardEscapeCoordinator();

  useEffect(() => coordinator.registerHandler({ enabled, handleEscape, priority }), [coordinator, enabled, handleEscape, priority]);
}
