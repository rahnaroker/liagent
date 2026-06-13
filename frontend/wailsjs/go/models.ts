export namespace main {
	
	export class CoverInfoView {
	    hasOriginal: boolean;
	    originalDataUri: string;
	    templateCount: number;
	    wallpaperDir: string;
	    applied: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CoverInfoView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasOriginal = source["hasOriginal"];
	        this.originalDataUri = source["originalDataUri"];
	        this.templateCount = source["templateCount"];
	        this.wallpaperDir = source["wallpaperDir"];
	        this.applied = source["applied"];
	    }
	}
	export class FindingView {
	    ruleId: string;
	    section: string;
	    context: string;
	    before: string;
	    after: string;
	
	    static createFrom(source: any = {}) {
	        return new FindingView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ruleId = source["ruleId"];
	        this.section = source["section"];
	        this.context = source["context"];
	        this.before = source["before"];
	        this.after = source["after"];
	    }
	}
	export class RuleStat {
	    id: string;
	    name: string;
	    category: string;
	    level: string;
	    count: number;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RuleStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.level = source["level"];
	        this.count = source["count"];
	        this.enabled = source["enabled"];
	    }
	}
	export class MetaInfo {
	    title: string;
	    author: string;
	    lang: string;
	    sections: number;
	    hasCover: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MetaInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.author = source["author"];
	        this.lang = source["lang"];
	        this.sections = source["sections"];
	        this.hasCover = source["hasCover"];
	    }
	}
	export class LoadResult {
	    path: string;
	    name: string;
	    meta: MetaInfo;
	    rules: RuleStat[];
	    findings: FindingView[];
	    total: number;
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LoadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.meta = this.convertValues(source["meta"], MetaInfo);
	        this.rules = this.convertValues(source["rules"], RuleStat);
	        this.findings = this.convertValues(source["findings"], FindingView);
	        this.total = source["total"];
	        this.truncated = source["truncated"];
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
	export class MetaEdit {
	    title: string;
	    authors: string;
	    translators: string;
	    lang: string;
	    seriesName: string;
	    seriesNumber: string;
	    publisher: string;
	    date: string;
	    isbn: string;
	    annotation: string;
	    keywords: string;
	
	    static createFrom(source: any = {}) {
	        return new MetaEdit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.authors = source["authors"];
	        this.translators = source["translators"];
	        this.lang = source["lang"];
	        this.seriesName = source["seriesName"];
	        this.seriesNumber = source["seriesNumber"];
	        this.publisher = source["publisher"];
	        this.date = source["date"];
	        this.isbn = source["isbn"];
	        this.annotation = source["annotation"];
	        this.keywords = source["keywords"];
	    }
	}
	

}

