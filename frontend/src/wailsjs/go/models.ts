export namespace main {
	
	export class ConfigDTO {
	    llama_server: string;
	    model: string;
	    backend: string;
	    context: number;
	    gpu_layers: number;
	    jev_port: number;
	    llama_port: number;
	    host: string;
	    lang: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.llama_server = source["llama_server"];
	        this.model = source["model"];
	        this.backend = source["backend"];
	        this.context = source["context"];
	        this.gpu_layers = source["gpu_layers"];
	        this.jev_port = source["jev_port"];
	        this.llama_port = source["llama_port"];
	        this.host = source["host"];
	        this.lang = source["lang"];
	    }
	}
	export class HardwareDTO {
	    name: string;
	    vendor: string;
	    vram_gb: number;
	    free_ram_gb: number;
	    backend: string;
	    gpu_layers: number;
	    summary: string;
	    summary_en: string;
	
	    static createFrom(source: any = {}) {
	        return new HardwareDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.vendor = source["vendor"];
	        this.vram_gb = source["vram_gb"];
	        this.free_ram_gb = source["free_ram_gb"];
	        this.backend = source["backend"];
	        this.gpu_layers = source["gpu_layers"];
	        this.summary = source["summary"];
	        this.summary_en = source["summary_en"];
	    }
	}
	export class ServerStatusDTO {
	    running: boolean;
	    starting: boolean;
	    cpu_fallback: boolean;
	    jev_port: number;
	    status: string;
	    active_backend: string;
	    active_device: string;
	    last_error?: string;
	    configured_context: number;
	    effective_context: number;
	    context_warning?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerStatusDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.starting = source["starting"];
	        this.cpu_fallback = source["cpu_fallback"];
	        this.jev_port = source["jev_port"];
	        this.status = source["status"];
	        this.active_backend = source["active_backend"];
	        this.active_device = source["active_device"];
	        this.last_error = source["last_error"];
	        this.configured_context = source["configured_context"];
	        this.effective_context = source["effective_context"];
	        this.context_warning = source["context_warning"];
	    }
	}
	export class InitialState {
	    app_name: string;
	    version: string;
	    cfg: ConfigDTO;
	    hw: HardwareDTO;
	    status: ServerStatusDTO;
	
	    static createFrom(source: any = {}) {
	        return new InitialState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.app_name = source["app_name"];
	        this.version = source["version"];
	        this.cfg = this.convertValues(source["cfg"], ConfigDTO);
	        this.hw = this.convertValues(source["hw"], HardwareDTO);
	        this.status = this.convertValues(source["status"], ServerStatusDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace selftest {
	
	export class FailureDetail {
	    id: string;
	    task: string;
	    generator: string;
	    log: string;
	    expected: string;
	    actual: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new FailureDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.task = source["task"];
	        this.generator = source["generator"];
	        this.log = source["log"];
	        this.expected = source["expected"];
	        this.actual = source["actual"];
	        this.reason = source["reason"];
	    }
	}
	export class GenStat {
	    total: number;
	    passed: number;
	    accuracy: number;
	
	    static createFrom(source: any = {}) {
	        return new GenStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.passed = source["passed"];
	        this.accuracy = source["accuracy"];
	    }
	}
	export class TaskStat {
	    total: number;
	    passed: number;
	    accuracy: number;
	
	    static createFrom(source: any = {}) {
	        return new TaskStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.passed = source["passed"];
	        this.accuracy = source["accuracy"];
	    }
	}
	export class SelfTestResult {
	    total_count: number;
	    passed_count: number;
	    accuracy: number;
	    task_stats: Record<string, TaskStat>;
	    generator_stats: Record<string, GenStat>;
	    failures: FailureDetail[];
	    avg_latency_ms: number;
	    avg_tok_s: number;
	    duration_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new SelfTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_count = source["total_count"];
	        this.passed_count = source["passed_count"];
	        this.accuracy = source["accuracy"];
	        this.task_stats = this.convertValues(source["task_stats"], TaskStat, true);
	        this.generator_stats = this.convertValues(source["generator_stats"], GenStat, true);
	        this.failures = this.convertValues(source["failures"], FailureDetail);
	        this.avg_latency_ms = source["avg_latency_ms"];
	        this.avg_tok_s = source["avg_tok_s"];
	        this.duration_ms = source["duration_ms"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

