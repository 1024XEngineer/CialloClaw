import type { ApprovalPendingNotification, MirrorOverviewUpdatedNotification } from "@cialloclaw/protocol";
import { NOTIFICATION_METHODS } from "@cialloclaw/protocol";

// subscribeTask 处理当前模块的相关逻辑。
export function subscribeTask(taskId: string, onMessage: (payload: unknown) => void) {
  const bridge = window.__CIALLOCLAW_NAMED_PIPE__;

  if (!bridge?.subscribe) {
    return () => {};
  }

  let disposed = false;
  let unsubscribe: (() => Promise<void>) | null = null;

  void bridge.subscribe(NOTIFICATION_METHODS.TASK_UPDATED, (payload) => {
    if (disposed) {
      return;
    }

    const message = payload as { params?: { task_id?: string } };
    if (!message.params?.task_id || message.params.task_id === taskId) {
      onMessage(payload);
    }
  }).then((subscription) => {
    if (disposed) {
      void subscription.unsubscribe();
      return;
    }

    unsubscribe = subscription.unsubscribe;
  });

  return () => {
    disposed = true;
    if (unsubscribe) {
      void unsubscribe();
    }
  };
}

export function subscribeMirrorOverviewUpdated(onMessage: (payload: MirrorOverviewUpdatedNotification) => void) {
  const bridge = window.__CIALLOCLAW_NAMED_PIPE__;

  if (!bridge?.subscribe) {
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
      if (disposed) {
        void subscription.unsubscribe();
        return;
      }

      unsubscribe = subscription.unsubscribe;
    });

  return () => {
    disposed = true;
    if (unsubscribe) {
      void unsubscribe();
    }
  };
}

export function subscribeApprovalPending(onMessage: (payload: ApprovalPendingNotification) => void) {
  const bridge = window.__CIALLOCLAW_NAMED_PIPE__;

  if (!bridge?.subscribe) {
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
      if (disposed) {
        void subscription.unsubscribe();
        return;
      }

      unsubscribe = subscription.unsubscribe;
    });

  return () => {
    disposed = true;
    if (unsubscribe) {
      void unsubscribe();
    }
  };
}
