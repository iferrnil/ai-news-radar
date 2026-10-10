import { EventEmitter } from "node:events";
const g = globalThis as unknown as { __newsBus?: EventEmitter };
export const bus: EventEmitter = g.__newsBus ?? (g.__newsBus = new EventEmitter().setMaxListeners(1000));
export type BusMsg = { event: "item" | "update" | "removed"; item?: unknown; id?: number };
export const publish = (m: BusMsg) => bus.emit("msg", m);
