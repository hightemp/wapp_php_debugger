// DTO types from spec/openapi.yaml. schema.gen.d.ts is generated (make generate); this file gives short names.
import type { components } from './schema.gen';

type Schemas = components['schemas'];

export type Level = Schemas['Level'];
export type Sapi = Schemas['Sapi'];
export type MessageType = Schemas['MessageType'];
export type Server = Schemas['Server'];
export type ServerConnection = Schemas['ServerConnection'];
export type ServerInput = Schemas['ServerInput'];
export type ServerPatch = Schemas['ServerPatch'];
export type ServerCheck = Schemas['ServerCheck'];
export type ServerCheckStep = Schemas['ServerCheckStep'];
export type Environment = Schemas['Environment'];
export type Project = Schemas['Project'];
export type DiscoveredProject = Schemas['DiscoveredProject'];
export type ProjectInput = Schemas['ProjectInput'];
export type ProjectPatch = Schemas['ProjectPatch'];
export type Request = Schemas['Request'];
export type RequestDetail = Schemas['RequestDetail'];
export type RequestPage = Schemas['RequestPage'];
export type Message = Schemas['Message'];
export type MessageDetail = Schemas['MessageDetail'];
export type MessagePage = Schemas['MessagePage'];
export type StreamEvent = Schemas['StreamEvent'];
export type Settings = Schemas['Settings'];
export type Problem = Schemas['Problem'];
