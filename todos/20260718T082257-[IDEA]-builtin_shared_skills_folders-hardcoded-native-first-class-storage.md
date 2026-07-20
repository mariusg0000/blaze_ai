# IDEA: Hardcoded built-in shared skills and folders

## WHAT MUST BE DONE
Explore adding direct, built-in support in BlazeAI for shared skills and folders, without relying on external synchronization or federation mechanisms. The application would natively understand that certain skills or folders have a shared origin and should be read from or written to a common location.

## WHY IT MUST BE DONE
The federation profile approach adds configuration complexity and still depends on external tools. A simpler, hardcoded support path may be sufficient for the immediate need: keeping selected skills such as ideas and personal-tracker unified across devices. This could be the pragmatic first step, with the federation model reserved for later generalization.

## HOW IT MUST BE DONE
Evaluate defining shared skills and folders declaratively in configuration or as a built-in concept in the runtime. The simplest form could be a configuration block pointing skill names to remote paths over SSH, with the app handling reads and writes directly. The goal is to reach a working unified data layer quickly, without overengineering the transport abstraction on the first pass.
