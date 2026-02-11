import { Component, OnDestroy, OnInit } from '@angular/core';
import { Observable, BehaviorSubject, timer, NEVER } from 'rxjs';
import { shareReplay, switchMap, map } from 'rxjs/operators';
import { NodeView, VisualizerService } from '../visualizer.service';

@Component({
  selector: 'app-nodes',
  templateUrl: './nodes.component.html',
  styleUrls: ['./nodes.component.scss']
})
export class NodesComponent implements OnInit, OnDestroy {

  nodes$: Observable<NodeView[]> | null = null;
  filteredNodes$: Observable<NodeView[]> | null = null;

  // Controls
  autoRefresh = true;
  refreshInterval = 5000;
  private refreshState$ = new BehaviorSubject<boolean>(true);

  // Filter state
  filterText = '';
  showGpuOnly = false;

  constructor(private service: VisualizerService) { }

  ngOnInit(): void {
    this.nodes$ = this.refreshState$.pipe(
      switchMap(isAuto => {
        if (isAuto) {
          return timer(0, this.refreshInterval).pipe(
            switchMap(() => this.service.getNodes())
          );
        } else {
          return NEVER;
        }
      }),
      shareReplay(1)
    );

    this.applyFilters();
  }

  ngOnDestroy(): void {
    // Async pipe handles unsubscription
  }

  toggleAutoRefresh(): void {
    this.autoRefresh = !this.autoRefresh;
    this.refreshState$.next(this.autoRefresh);
  }

  applyFilters(): void {
    if (!this.nodes$) return;

    this.filteredNodes$ = this.nodes$.pipe(
      map(nodes => {
        if (!nodes) return [];
        return nodes.filter(n => {
          const matchesText = n.name.toLowerCase().includes(this.filterText.toLowerCase());
          const matchesGpu = this.showGpuOnly ? (n.gpuSlots && n.gpuSlots.length > 0) : true;
          return matchesText && matchesGpu;
        });
      })
    );
  }

  onFilterChange(text: string): void {
    this.filterText = text;
    this.applyFilters();
  }

  toggleGpuFilter(): void {
    this.showGpuOnly = !this.showGpuOnly;
    this.applyFilters();
  }
}
