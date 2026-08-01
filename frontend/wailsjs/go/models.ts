export namespace port {
	
	export class AgentConfig {
	    ID: string;
	    Name: string;
	    Provider: string;
	    Model: string;
	    BaseURL: string;
	    APIKey: string;
	    SystemPrompt: string;
	    Temperature: number;
	
	    static createFrom(source: any = {}) {
	        return new AgentConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.Provider = source["Provider"];
	        this.Model = source["Model"];
	        this.BaseURL = source["BaseURL"];
	        this.APIKey = source["APIKey"];
	        this.SystemPrompt = source["SystemPrompt"];
	        this.Temperature = source["Temperature"];
	    }
	}
	export class Shortcut {
	    ID: string;
	    Label: string;
	    Keys: string[];
	    Enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Shortcut(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Label = source["Label"];
	        this.Keys = source["Keys"];
	        this.Enabled = source["Enabled"];
	    }
	}
	export class AppSettings {
	    Theme: string;
	    Language: string;
	    Shortcuts: Shortcut[];
	    AutoStartListening: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Theme = source["Theme"];
	        this.Language = source["Language"];
	        this.Shortcuts = this.convertValues(source["Shortcuts"], Shortcut);
	        this.AutoStartListening = source["AutoStartListening"];
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
	export class AudioDevice {
	    ID: string;
	    Name: string;
	    IsDefault: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AudioDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.IsDefault = source["IsDefault"];
	    }
	}
	export class Message {
	    Role: string;
	    Text: string;
	    Timestamp: number;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Role = source["Role"];
	        this.Text = source["Text"];
	        this.Timestamp = source["Timestamp"];
	    }
	}
	export class Session {
	    ID: string;
	    ChatHistory: Message[];
	    StartedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.ChatHistory = this.convertValues(source["ChatHistory"], Message);
	        this.StartedAt = source["StartedAt"];
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

