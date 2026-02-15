export interface IngestRequest {
    user_id: string;
    content: string;
    role: "user" | "assistant";
}

export interface IngestResponse {
    episode_ids: string[];
    segments: number;
    message: string;
}

export interface QueryRequest {
    user_id: string;
    query: string;
    token_budget?: number;
}

export interface RetrievalResult {
    episode?: any; // Define more specifically if needed
    graph_facts?: any[];
    score: number;
    source: string;
}

export interface QueryResponse {
    context: string;
    sources: RetrievalResult[];
    tokens_used: number;
    token_budget: number;
    dig_scores?: Record<string, number>;
}

export interface HealthResponse {
    status: string;
    version: string;
    services: Record<string, string>;
    timestamp: string;
}

const API_BASE = "http://localhost:8080/api/v1";

export const cmaApi = {
    async ingest(userId: string, content: string, role: "user" | "assistant"): Promise<IngestResponse> {
        const res = await fetch(`${API_BASE}/ingest`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ user_id: userId, content, role }),
        });
        if (!res.ok) throw new Error(`Ingest failed: ${res.statusText}`);
        return res.json();
    },

    async query(userId: string, query: string): Promise<QueryResponse> {
        const res = await fetch(`${API_BASE}/query`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ user_id: userId, query }),
        });
        if (!res.ok) throw new Error(`Query failed: ${res.statusText}`);
        return res.json();
    },

    async health(): Promise<HealthResponse> {
        const res = await fetch(`http://localhost:8080/health`);
        if (!res.ok) throw new Error(`Health check failed: ${res.statusText}`);
        return res.json();
    },

    async getHippocampus(userId: string): Promise<{ episodes: any[], stats: any }> {
        const res = await fetch(`${API_BASE}/hippocampus?user_id=${userId}`);
        if (!res.ok) throw new Error(`Hippocampus fetch failed: ${res.statusText}`);
        return res.json();
    },

    async getNeocortex(userId: string): Promise<{ nodes: number, edges: number }> {
        const res = await fetch(`${API_BASE}/neocortex?user_id=${userId}`);
        if (!res.ok) throw new Error(`Neocortex fetch failed: ${res.statusText}`);
        return res.json();
    },

    async getSystemLogs(): Promise<{ ts: string, level: string, module: string, msg: string }[]> {
        const res = await fetch(`${API_BASE}/system/logs`);
        if (!res.ok) throw new Error(`Logs fetch failed: ${res.statusText}`);
        return res.json();
    },

    async getWorkspaceContext(userId: string): Promise<any> {
        const res = await fetch(`${API_BASE}/workspace/context?user_id=${userId}`);
        if (!res.ok) throw new Error(`Workspace fetch failed: ${res.statusText}`);
        return res.json();
    }
};
