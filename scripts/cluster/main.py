#!/usr/bin/env python3
"""
Monitor de Cluster Go - Aplicación Principal
Punto de entrada de la aplicación
"""

import sys
import os

# Agregar el directorio actual al path para imports
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

from ui import GoClusterMonitorApp

def main():
    """Función principal de la aplicación"""
    try:
        app = GoClusterMonitorApp()
        app.run()
    except KeyboardInterrupt:
        print("\n👋 Aplicación cerrada por el usuario")
    except Exception as e:
        print(f"❌ Error crítico: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()