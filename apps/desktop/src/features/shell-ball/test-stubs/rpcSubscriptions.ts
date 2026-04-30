import type {
  ApprovalPendingNotification,
  DeliveryReadyNotification,
  MirrorOverviewUpdatedNotification,
  TaskRuntimeNotification,
  TaskSteeredNotification,
  TaskUpdatedNotification,
} from "@cialloclaw/protocol";

function noopUnsubscribe() {
  return () => {};
}

export function subscribeApprovalPending(_listener?: (payload: ApprovalPendingNotification) => void) {
  return noopUnsubscribe();
}

export function subscribeDeliveryReady(_listener?: (payload: DeliveryReadyNotification) => void) {
  return noopUnsubscribe();
}

export function subscribeMirrorOverviewUpdated(_listener?: (payload: MirrorOverviewUpdatedNotification) => void) {
  return noopUnsubscribe();
}

export function subscribeTask(_taskId?: string, _listener?: (payload: TaskUpdatedNotification) => void) {
  return noopUnsubscribe();
}

export function subscribeTaskUpdated(_listener?: (payload: TaskUpdatedNotification) => void) {
  return noopUnsubscribe();
}

export function subscribeTaskRuntime(_taskId?: string, _listener?: (payload: TaskSteeredNotification | TaskRuntimeNotification) => void) {
  return noopUnsubscribe();
}

export function subscribeAllTaskRuntime(_listener?: (payload: TaskSteeredNotification | TaskRuntimeNotification) => void) {
  return noopUnsubscribe();
}
