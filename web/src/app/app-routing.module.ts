import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { DashboardComponent } from './dashboard/dashboard.component';
import { JobsComponent } from './jobs/jobs.component';
import { NodesComponent } from './nodes/nodes.component';
import { QueuesComponent } from './queues/queues.component';

const routes: Routes = [
  { path: '', redirectTo: '/dashboard', pathMatch: 'full' },
  { path: 'dashboard', component: DashboardComponent },
  { path: 'jobs', component: JobsComponent },
  { path: 'nodes', component: NodesComponent },
  { path: 'queues', component: QueuesComponent },
];

@NgModule({
  imports: [RouterModule.forRoot(routes)],
  exports: [RouterModule]
})
export class AppRoutingModule { }
