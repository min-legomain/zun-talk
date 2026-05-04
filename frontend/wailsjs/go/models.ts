export namespace main {
	
	export class CharacterInfo {
	    key: string;
	    name: string;
	    speakerId: number;
	
	    static createFrom(source: any = {}) {
	        return new CharacterInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.speakerId = source["speakerId"];
	    }
	}

}

