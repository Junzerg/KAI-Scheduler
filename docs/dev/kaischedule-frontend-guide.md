# KaiSchedule Frontend Integration Guide (Kubeflow)

This guide provides the necessary information to integrate the **KaiSchedule** frontend into the Kubeflow Dashboard. Copy this content to the KaiSchedule project as a reference for developers.

---

## 1. UI Style Consistency (Styling)

Kubeflow provides a set of standard CSS variables. Reference the stylesheet provided by the main dashboard in your HTML:

```html
<!-- Reference the style exposed by the main dashboard -->
<link rel="stylesheet" href="/kubeflow-palette.css" />

<style>
  body {
    background-color: var(--sidebar-color); /* Sidebar background color */
    color: #212121;
    font-family: "Roboto", sans-serif;
  }
  .action-button {
    background-color: var(--accent-color); /* Standard blue button */
    color: white;
  }
  .card {
    border: 1px solid var(--border-color);
    background: white;
  }
</style>
```

**Core Variables Reference:**

- `--accent-color`: `#007dfc` (Primary Action/Highlight)
- `--kubeflow-color`: `#003c75` (Signature Deep Blue)
- `--sidebar-color`: `#f8fafb` (Page Background/Sidebar)
- `--border-color`: `#f4f4f6` (Border Color)

---

## 2. Namespace Awareness (Multi-tenancy)

The main dashboard loads this page via an iframe and communicates using an SDK. You need to listen for namespace change events to display tasks for that specific namespace.

### Include the Script

```html
<script src="/dashboard_lib.bundle.js"></script>
```

### Core Interaction Logic

```javascript
let currentNamespace = "default";

window.addEventListener("DOMContentLoaded", () => {
  // Check if running within the Kubeflow environment
  if (
    !window.centraldashboard ||
    !window.centraldashboard.CentralDashboardEventHandler
  ) {
    console.warn("NotInKubeflow: Running in standalone mode.");
    return;
  }

  const sdk = window.centraldashboard.CentralDashboardEventHandler;

  /**
   * Initialize SDK
   * @param {Object} cdeh - Event Handler Instance
   * @param {Boolean} isIframed - Whether it is embedded in the main dashboard
   */
  sdk.init((cdeh, isIframed) => {
    // 1. Listen for namespace changes
    cdeh.onNamespaceSelected = (namespace) => {
      if (!namespace) return;
      console.log(`Switching to namespace: ${namespace}`);
      currentNamespace = namespace;

      // Trigger your business logic here, e.g.:
      // fetchTasks(currentNamespace);
    };

    // 2. Adjust layout if embedded in an iframe (e.g., hide own top navigation)
    if (isIframed) {
      document.body.classList.add("is-iframed");
    }
  });
});
```

---

## 3. Development Recommendations

1.  **Stateful URLs**: Support access via query parameters, such as `/kaischedule/?namespace=my-user`. This helps with independent debugging.
2.  **Hide Redundant UI**: In iframe mode, hide your own sidebar or main title, as Kubeflow provides a unified shell.
3.  **Authentication**: You don't need to handle Tokens manually for API requests. Istio verifies user identity, and your backend can retrieve the logged-in user's email from the `kubeflow-userid` header.

---

## 4. Integration Verification

After deployment, ensure the following in the Kubeflow cluster:

1.  **Istio Routing**: Confirm the `/kaischedule/` path is routed to this service via a `VirtualService`.
2.  **Menu Registration**: Confirm the `dashboard-config` ConfigMap includes the KaiSchedule entry.
