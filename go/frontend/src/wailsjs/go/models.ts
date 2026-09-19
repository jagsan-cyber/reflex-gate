export namespace main {
	
	export class ConfigDTO {
	    llama_server: string;
	    model: string;
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
	    jev_port: number;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerStatusDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.starting = source["starting"];
	        this.jev_port = source["jev_port"];
	        this.status = source["status"];
	    }
	}
	export class InitialState {
	    cfg: ConfigDTO;
	    hw: HardwareDTO;
	    status: ServerStatusDTO;
	
	    static createFrom(source: any = {}) {
	        return new InitialState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
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

