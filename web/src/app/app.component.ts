import { Component, ViewChild } from '@angular/core';
import { MatSidenav } from '@angular/material/sidenav';
import { NamespaceService } from './namespace.service';
import { RefreshService } from './refresh.service';

@Component({
  selector: 'app-root',
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.scss']
})
export class AppComponent {
  title = 'console';
  namespaces: string[] = ['default', 'kube-system']; // TODO: Fetch from backend dynamically in future
  selectedNamespace = '';

  @ViewChild('sidenav') sidenav!: MatSidenav;

  constructor(
    private namespaceService: NamespaceService,
    public refreshService: RefreshService
  ) { }

  onNamespaceChange(namespace: string): void {
    this.selectedNamespace = namespace;
    this.namespaceService.setNamespace(namespace);
  }
}
