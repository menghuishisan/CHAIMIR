import type { JsonObject, RuntimeSnapshot, SimEvent, SimInitParams, SimPackageDescriptor } from '../types';

export type WorkerRequest =
  | { type: 'init'; requestId: number; builtinCode: string; initParams: SimInitParams; seed: number }
  | { type: 'step'; requestId: number }
  | { type: 'inject'; requestId: number; eventType: string; payload: JsonObject; target?: string }
  | { type: 'back'; requestId: number }
  | { type: 'reset'; requestId: number };

export type WorkerResponse =
  | { type: 'ready'; requestId: number; descriptor: SimPackageDescriptor; snapshot: RuntimeSnapshot }
  | { type: 'snapshot'; requestId: number; snapshot: RuntimeSnapshot; event?: SimEvent }
  | { type: 'error'; requestId: number; message: string };
