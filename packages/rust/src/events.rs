use serde_json::{Map, Value};
use std::sync::{Arc, Mutex};
use std::time::Instant;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum EventKind {
    EnvCreated,
    EnvAttached,
    ProfileMaterialized,
    CredentialsCopied,
    CredentialsRefreshed,
    EnvDestroyed,
    RegistryRead,
    RegistryWritten,
    Error,
}

impl EventKind {
    pub fn as_str(&self) -> &'static str {
        match self {
            EventKind::EnvCreated => "env.created",
            EventKind::EnvAttached => "env.attached",
            EventKind::ProfileMaterialized => "profile.materialized",
            EventKind::CredentialsCopied => "credentials.copied",
            EventKind::CredentialsRefreshed => "credentials.refreshed",
            EventKind::EnvDestroyed => "env.destroyed",
            EventKind::RegistryRead => "registry.read",
            EventKind::RegistryWritten => "registry.written",
            EventKind::Error => "error",
        }
    }
}

#[derive(Debug, Clone)]
pub struct Event {
    pub ts_ms: u64,
    pub kind: EventKind,
    pub data: Map<String, Value>,
}

impl Event {
    pub fn to_wire(&self) -> Map<String, Value> {
        let mut m = Map::new();
        m.insert("ts_ms".into(), Value::from(self.ts_ms));
        m.insert("kind".into(), Value::from(self.kind.as_str()));
        for (k, v) in &self.data {
            m.insert(k.clone(), v.clone());
        }
        m
    }
}

pub type EventSink = Arc<dyn Fn(&Event) + Send + Sync>;

pub(crate) struct EventLog {
    origin: Instant,
    events: Mutex<Vec<Event>>,
    sink: Option<EventSink>,
}

impl EventLog {
    pub fn new(sink: Option<EventSink>) -> Self {
        Self {
            origin: Instant::now(),
            events: Mutex::new(Vec::new()),
            sink,
        }
    }

    pub fn emit(&self, kind: EventKind, data: Map<String, Value>) {
        let ts_ms = self.origin.elapsed().as_millis() as u64;
        let event = Event { ts_ms, kind, data };
        if let Some(sink) = &self.sink {
            (sink)(&event);
        }
        if let Ok(mut guard) = self.events.lock() {
            guard.push(event);
        }
    }

    pub fn snapshot(&self) -> Vec<Event> {
        self.events.lock().map(|g| g.clone()).unwrap_or_default()
    }
}
