package main

var webHomepageTemplate = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
    <title>toglacier</title>

    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0-beta/css/bootstrap.min.css" integrity="sha384-/Y6pD6FV/Vv2HJnA6t+vslU6fwYXjCFtcEpHbNJ0lyAFsXTsjBbfaDjzALeQsN6M" crossorigin="anonymous">
  </head>
  <body>
    <nav class="navbar navbar-dark bg-primary">
      <a class="navbar-brand" href="#">toglacier</a>
    </nav>
    
    <div class="container-fluid">
      <div class="row">
        <div class="col">
          <div class="alert alert-danger alert-dismissible fade show mt-2" role="alert">
            <button type="button" class="close" data-dismiss="alert" aria-label="Close">
              <span aria-hidden="true">&times;</span>
            </button>
            <p></p>
          </div>
        </div>
      </div>

      <div class="row">
        <div class="col">
          <ul class="nav nav-tabs mt-3 mb-3" role="tablist">
            <li class="nav-item">
              <a class="nav-link active" data-toggle="tab" href="#backups" role="tab">Backups</a>
            </li>
            <li class="nav-item">
              <a class="nav-link" data-toggle="tab" href="#settings" role="tab">Settings</a>
            </li>
          </ul>
        </div>
      </div>

      <div class="row">
        <div class="col">
          <div class="tab-content">
            <div class="tab-pane fade show active" id="backups" role="tabpanel" aria-labelledby="backups-tab">
              <table class="table">
                <thead>
                  <tr>
                    <th>#</th>
                    <th>Date</th>
                  </tr>
                </thead>
                <tbody id="backups-results"></tbody>
              </table>
            </div>
            <div class="tab-pane fade" id="settings" role="tabpanel" aria-labelledby="settings-tab">
              <form>
                <div class="form-group">
                  <label for="paths">Paths</label>
                  <textarea id="paths" class="form-control" aria-describedby="pathsHelp" placeholder="Enter paths"></textarea>
                  <small id="pathsHelp" class="form-text text-muted">
                    Paths is the list of all locations that you want to backup.
                    It could be a directory or a specific file. Add one path per
                    line.
                  </small>
                </div>
              </form>
            </div>
          </div>
        </div>
      </div>
    </div>

    <script src="https://code.jquery.com/jquery-3.2.1.min.js" integrity="sha256-hwg4gsxgFZhOsEEamdOYGBf13FyQuiTwlAQgxVSNgt4=" crossorigin="anonymous"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/popper.js/1.11.0/umd/popper.min.js" integrity="sha384-b/U6ypiBEHpOf/4+1nzFpr53nxSS+GLCkfwBdFNTxtclqqenISfwAzpKaMNFNmj4" crossorigin="anonymous"></script>
    <script src="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0-beta/js/bootstrap.min.js" integrity="sha384-h0AbiXch4ZDo7tp9hKZ4TsHbi047NrKGLO3SEJAg45jXxnGIfYzk4Si90RDIqNm1" crossorigin="anonymous"></script>

    <script type="text/javascript">
      function backups() {
        $.ajax("/backups")

          .done(function(backups, textStatus, jqXHR) {
            $("#backups-results").empty();
            $.each(JSON.parse(backups), function(index, backup) {
              $("#backups-results").append(
                $("<tr>").append(
                  $("<th>")
                    .attr("scope", "row")
                    .text(index),
                  $("<td>").text(backup.Backup.CreatedAt)
                )
              );
            });
          })

          .fail(function(jqXHR, textStatus, errorThrown) {
            $(".alert-danger > p").text("Error loading backups");
            $(".alert").alert();
          })

          .always(function() {
            $(".alert-danger > p").text("");
            $(".alert").alert("close");
            setTimeout(backups, 5000);
          });
      }

      function config() {
        $.ajax("/config")

          .done(function(data, textStatus, jqXHR) {
            var config = JSON.parse(data);

            $("#paths").val("");
            $.each(config.paths, function(index, path) {
              $("#paths").val(path + "\n");
            });
          })

          .fail(function(jqXHR, textStatus, errorThrown) {
            $(".alert-danger > p").text("Error loading config");
            $(".alert").alert();
          })

          .always(function() {
            $(".alert-danger > p").text("");
            $(".alert").alert("close");
            setTimeout(backups, 5000);
          });
      }

      $(function() {
        backups();
        config();
      });
    </script>
  </body>
</html>
`
