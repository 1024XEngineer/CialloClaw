import type {
  ApprovalPendingNotification,
  DeliveryReadyNotification,
  MirrorOverviewUpdatedNotification,
  TaskRuntimeNotification,
  TaskSteeredNotification,
  TaskUpdatedNotification,
} from "@cialloclaw/protocol";
import { isRpcChannelUnavailable } from "./fallback";
import { NOTIFICATION_METHODS } from "./protocolConstants";

const NAMED_PIPE_SUBSCRIPTION_UNAVAILABLE_COOLDOWN_MS = 10_000;

let namedPipeSubscriptionsUnavailableAt = 0;

function clearNamedPipeSubscriptionFailureState() {
  namedPipeSubscriptionsUnavailableAt = 0;
}

function shouldShortCircuitNamedPipeSubscriptions() {
  return (
    namedPipeSubscriptionsUnavailableAt !== 0 &&
    Date.now() - namedPipeSubscriptionsUnavailableAt < NAMED_PIPE_SUBSCRIPTION_UNAVAILABLE_COOLDOWN_MS
  );
}

function handleNamedPipeSubscriptionFailure(error: unknown) {
  if (isRpcChannelUnavailable(error)) {
    namedPipeSubscriptionsUnavailableAt = Date.now();
    return;
  }

  console.warn("rpc subscription failed", error);
}

// subscribeTask keeps task-centric views aligned with task.updated notifications.
export function subscribeTask(taskId: string, onMessage: (payload: unknown) => void) {
  const bridge = window.__CIALLOCLAW_NAMED_PIPE__;

  if (!bridge?.subscribe || shouldShortCircuitNamedPipeSubscriptions()) {
    return () => {};
  }

  let disposed = false;
  let unsubscribe: (() => Promise<void>) | null = null;

  void bridge
    .subscribe(NOTIFICATION_METHODS.TASK_UPDATED, (payload) => {
      if (disposed) {
        return;
      }

      const message = payload as { params?: { task_id?: string } };
      if (!message.params?.task_id || message.params.task_id === taskId) {
        onMessage(payload);
      }
    })
    .then((subscription) => {
      clearNamedPipeSubscriptionFailureState();
      if (disposed) {
        void subscription.unsubscribe();
        return;
      }

      unsubscribe = subscription.unsubscribe;
    })
    .catch(handleNamedPipeSubscriptionFailure);

  return () => {
    disposed = true;
    if (unsubscribe) {
      void unsubscribe();
    }
  };
}

export function subscribeTaskUpdated(onMessage: (payload: TaskUpdatedNotification) => void) {
  const bridge = window.__CIALLOCLAW_NAMED_PIPE__;

  if (!bridge?.subscribe || shouldShortCircuitNamedPipeSubscriptions()) {
    return () => {};
  }

  let disposed = false;
  let unsubscribe: (() => Promise<void>) | null = null;

  void bridge
    .subscribe(NOTIFICATION_METHODS.TASK_UPDATED, (payload) => {
      if (!disposed) {
        const message = payload as { params?: TaskUpdatedNotification };
        if (message.params) {
          onMessage(message.params);
        }
      }
    })
    .then((subscription) => {
      clearNamedPipeSubscriptionFailureState();
      if (disposed) {
        void subscription.unsubscribe();
        return;
      }

      unsubscribe = subscription.unsubscribe;
    })
    .catch(handleNamedPipeSubscriptionFailure);

  return () => {
    disposed = true;
    if (unsubscribe) {
      void unsubscribe();
    }
  };
}

export function subscribeMirrorOverviewUpdated(onMessage: (payload: MirrorOverviewUpdatedNotification) => void) {
  const bridge = window.__CIALLOCLAW_NAMED_PIPE__;

  if (!bridge?.subscribe || shouldShortCircuitNamedPipeSubscriptions()) {
    return () => {};
  }

  let disposed = false;
  let unsubscribe: (() => Promise<void>) | null = null;

  void bridge
    .subscribe(NOTIFICATION_METHODS.MIRROR_OVERVIEW_UPDATED, (payload) => {
      if (!disposed) {
        const message = payload as { params?: MirrorOverviewUpdatedNotification };
        if (message.params) {
          onMessage(message.params);
        }
      }
    })
    .then((subscription) => {
      clearNamedPipeSubscriptionFailureState();
      if (disposed) {
        void subscription.unsubscribe();
        return;
      }

      unsubscribe = subscription.unsubscribe;
    })
    .catch(handleNamedPipeSubscriptionFailure);

  return () => {
    disposed = true;
    if (unsubscribe) {
      void unsubscribe();
    }
  };
}

export function subscribeApprovalPending(onMessage: (payload: ApprovalPendingNotification) => void) {
  const bridge = window.__CIALLOCLAW_NAMED_PIPE__;

  if (!bridge?.subscribe || shouldShortCircuitNamedPipeSubscriptions()) {
    return () => {};
  }

  let disposed = false;
  let unsubscribe: (() => Promise<void>) | null = null;

  void bridge
    .subscribe(NOTIFICATION_METHODS.APPROVAL_PENDING, (payload) => {
      if (!disposed) {
        const message = payload as { params?: ApprovalPendingNotification };
        if (message.params) {
          onMessage(message.params);
        }
      }
    })
    .then((subscription) => {
      clearNamedPipeSubscriptionFailureState();
      if (disposed) {
        void subscription.unsubscribe();
        return;
      }

      unsubscribe = subscription.unsubscribe;
    })
    .catch(handleNamedPipeSubscriptionFailure);

  return () => {
    disposed = true;
    if (unsubscribe) {
      void unsubscribe();
    }
  };
}

export function subscribeDeliveryReady(onMessage: (payload: DeliveryReadyNotification) => void) {
  const bridge = window.__CIALLOCLAW_NAMED_PIPE__;

  if (!bridge?.subscribe || shouldShortCircuitNamedPipeSubscriptions()) {
    return () => {};
  }

  let disposed = false;
  let unsubscribe: (() => Promise<void>) | null = null;

  void bridge
    .subscribe(NOTIFICATION_METHODS.DELIVERY_READY, (payload) => {
      if (!disposed) {
        const message = payload as { params?: DeliveryReadyNotification };
        if (message.params) {
          onMessage(message.params);
        }
      }
    })
    .then((subscription) => {
      clearNamedPipeSubscriptionFailureState();
      if (disposed) {
        void subscription.unsubscribe();
        return;
      }

      unsubscribe = subscription.unsubscribe;
    })
    .catch(handleNamedPipeSubscriptionFailure);

  return () => {
    disposed = true;
    if (unsubscribe) {
      void unsubscribe();
    }
  };
}

// subscribeTaskRuntime keeps task detail views aligned with task.steered and
// loop.* notifications without promoting these runtime payloads into formal
// frontend state. Callers still re-read the task-centric queries after each hit.
export function subscribeTaskRuntime(taskId: string, onMessage: (payload: TaskSteeredNotification | TaskRuntimeNotification) => void) {
  const bridge = window.__CIALLOCLAW_NAMED_PIPE__;

  if (!bridge?.subscribe || shouldShortCircuitNamedPipeSubscriptions()) {
    return () => {};
  }

  let disposed = false;
  const unsubscribes: Array<() => Promise<void>> = [];

  const methods = [
    NOTIFICATION_METHODS.TASK_STEERED,
    NOTIFICATION_METHODS.LOOP_STARTED,
    NOTIFICATION_METHODS.LOOP_ROUND_STARTED,
    NOTIFICATION_METHODS.LOOP_RETRYING,
    NOTIFICATION_METHODS.LOOP_COMPACTED,
    NOTIFICATION_METHODS.LOOP_ROUND_COMPLETED,
    NOTIFICATION_METHODS.LOOP_COMPLETED,
    NOTIFICATION_METHODS.LOOP_FAILED,
  ] as const;

  for (const method of methods) {
    void bridge
      .subscribe(method, (payload) => {
        if (disposed) {
          return;
        }

        const message = payload as { params?: TaskSteeredNotification | TaskRuntimeNotification };
        if (message.params?.task_id === taskId) {
          onMessage(message.params);
        }
      })
      .then((subscription) => {
        clearNamedPipeSubscriptionFailureState();
        if (disposed) {
          void subscription.unsubscribe();
          return;
        }

        unsubscribes.push(subscription.unsubscribe);
      })
      .catch(handleNamedPipeSubscriptionFailure);
  }

  return () => {
    disposed = true;
    for (const unsubscribe of unsubscribes) {
      void unsubscribe();
    }
  };
}
