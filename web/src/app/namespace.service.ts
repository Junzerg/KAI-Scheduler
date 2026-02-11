import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';

@Injectable({
    providedIn: 'root'
})
export class NamespaceService {
    private selectedNamespaceSubject = new BehaviorSubject<string>(''); // '' means all namespaces
    selectedNamespace$: Observable<string> = this.selectedNamespaceSubject.asObservable();

    constructor() { }

    setNamespace(namespace: string): void {
        this.selectedNamespaceSubject.next(namespace);
    }

    getCurrentNamespace(): string {
        return this.selectedNamespaceSubject.value;
    }
}
